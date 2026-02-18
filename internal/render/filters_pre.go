package render

import (
	"fmt"
	"regexp"
	"strings"
)

// 1) Normalize whitespace and editor line breaks
func normalizeWhitespaceAndBreaks(content string) string {
	content = strings.NewReplacer(
		"\u00A0", " ", // NBSP
		"\u2002", " ", // en space
		"\u2003", " ", // em space
		"\u2007", " ", // figure space
		"\u202F", " ", // narrow NBSP
		"&nbsp;", " ",
		"&#160;", " ",
		"\r\n", "\n",
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
	).Replace(content)
	return content
}

// 2) Hide single-line CSS rules accidentally pasted (outside code)
func stripStyleSnippets(content string) string {
	return protectPreBlocks(content, func(s string) string {
		rePre := regexp.MustCompile(`(?m)^\s*pre\s*code\s*\{[^}]*\}\s*$`)
		s = rePre.ReplaceAllString(s, "")
		reToken := regexp.MustCompile(`(?m)^\s*\.[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*\s*\{[^}]*\}\s*$`)
		s = reToken.ReplaceAllString(s, "")
		return s
	})
}

// 3) Remove <more--> marker (first occurrence, with or without space)
func replaceMoreTag(content string) string {
	markers := []string{"<more-->", "<more -->", "&lt;more--&gt;", "&lt;more --&gt;"}
	for _, mk := range markers {
		if idx := strings.Index(content, mk); idx != -1 {
			return content[:idx] + content[idx+len(mk):]
		}
	}
	return content
}

//  4. Unwrap list-like containers produced by WYSIWYG
//     Example:
//
//     <div style="...">- Item</div>
//     <div style="...">- Item 2</div>
//     -> "- Item\n- Item 2\n\n"
//
//     Also handles ordered items "1. Item".
func unwrapListLikeContainers(content string) string {
	// Handle div and p tags separately since Go regex doesn't support backreferences like \1
	// Updated to handle cases where there's no space after the list marker
	divRe := regexp.MustCompile(`(?is)<div\b[^>]*>\s*([\-\*\+]\s*|\d+\.\s*)([\s\S]*?)</div>`)
	pRe := regexp.MustCompile(`(?is)<p\b[^>]*>\s*([\-\*\+]\s*|\d+\.\s*)([\s\S]*?)</p>`)
	
	replaceFunc := func(matches []string) string {
		if len(matches) < 3 {
			return matches[0]
		}
		marker := strings.TrimSpace(matches[1])
		text := strings.TrimSpace(matches[2])
		// Ensure proper spacing between marker and text
		if marker != "" && !strings.HasSuffix(marker, " ") {
			marker += " "
		}
		// If the text accidentally ended up with an extra closing tag from inline HTML,
		// keep it verbatim—markdown will pass HTML through.
		return "\n" + marker + text + "\n"
	}
	
	s := divRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := divRe.FindStringSubmatch(m)
		return replaceFunc(sub)
	})
	
	s = pRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := pRe.FindStringSubmatch(m)
		return replaceFunc(sub)
	})

	// Merge consecutive unwrapped items nicely and ensure a blank line at end.
	s = regexp.MustCompile(`(?m)\n([-\*\+] |\d+\. ).+\n(?:(?:[-\*\+] |\d+\. ).+\n)+`).ReplaceAllStringFunc(s, func(block string) string {
		return strings.TrimRight(block, "\n") + "\n\n"
	})
	return s
}

// 5) Ensure there is a blank line before any list that follows text.
func ensureListSeparation(content string) string {
	re := regexp.MustCompile(`(?m)([^\n])\n([ \t]*)([-*+]|\d+\.)\s+`)
	return re.ReplaceAllString(content, "$1\n\n$2$3 ")
}

//  6. Convert headings/quotes that appear inside plain HTML containers and
//     add blank lines after block containers so markdown resumes cleanly.
func preprocessLooseMarkdownHTML(content string) string {
	// Blank line after common block containers
	closeBlock := regexp.MustCompile(`(?is)</(div|figure|section|table|blockquote|p)>\s*`)
	content = closeBlock.ReplaceAllString(content, "</$1>\n\n")

	// Convert raw top-level markdown headings (if WYSIWYG collapsed them)
	reTopH3 := regexp.MustCompile(`(?m)^[ \t]*###[ \t]+(.+)$`)
	content = reTopH3.ReplaceAllString(content, `<h3>$1</h3>`)
	reTopH2 := regexp.MustCompile(`(?m)^[ \t]*##[ \t]+(.+)$`)
	content = reTopH2.ReplaceAllString(content, `<h2>$1</h2>`)
	reTopH1 := regexp.MustCompile(`(?m)^[ \t]*#[ \t]+(.+)$`)
	content = reTopH1.ReplaceAllString(content, `<h1>$1</h1>`)

	// Multi-line blockquotes to a single <blockquote><p>...</p></blockquote>
	content = processBlockquotes(content)

	// Paragraph-wrapped headings
	reH3 := regexp.MustCompile(`(?is)<p[^>]*>\s*###\s+(.+?)\s*</p>`)
	content = reH3.ReplaceAllString(content, `<h3>$1</h3>`)
	reH2 := regexp.MustCompile(`(?is)<p[^>]*>\s*##\s+(.+?)\s*</p>`)
	content = reH2.ReplaceAllString(content, `<h2>$1</h2>`)
	reH1 := regexp.MustCompile(`(?is)<p[^>]*>\s*#\s+(.+?)\s*</p>`)
	content = reH1.ReplaceAllString(content, `<h1>$1</h1>`)

	// Paragraph/div wrapped list markers -> plain lines
	rePUL := regexp.MustCompile(`(?is)<p[^>]*>\s*([ \t]*)([-*+])\s+(.+?)\s*</p>`)
	content = rePUL.ReplaceAllString(content, "\n$1$2 $3\n")
	reDivUL := regexp.MustCompile(`(?is)<div[^>]*>\s*([ \t]*)([-*+])\s+(.+?)\s*</div>`)
	content = reDivUL.ReplaceAllString(content, "\n$1$2 $3\n")
	
	rePOL := regexp.MustCompile(`(?is)<p[^>]*>\s*([ \t]*)(\d+)\.\s+(.+?)\s*</p>`)
	content = rePOL.ReplaceAllString(content, "\n$1$2. $3\n")
	reDivOL := regexp.MustCompile(`(?is)<div[^>]*>\s*([ \t]*)(\d+)\.\s+(.+?)\s*</div>`)
	content = reDivOL.ReplaceAllString(content, "\n$1$2. $3\n")

	// Indent tweaks: normalize 2-space indented nested items to 4 spaces
	indent2UL := regexp.MustCompile(`(?m)^[ ]{2}([-*+]\s+)`)
	content = indent2UL.ReplaceAllString(content, `    $1`)
	indent2OL := regexp.MustCompile(`(?m)^[ ]{2}(\d+\.\s+)`)
	content = indent2OL.ReplaceAllString(content, `    $1`)

	// Unwrap paragraphs that contain Markdown emphasis to let the renderer parse them
	rePEmph := regexp.MustCompile(`(?is)<p[^>]*>\s*([^<>]*?(\*\*.+?\*\*|__.+?__|\*[^*]+?\*|_[^_]+?_)\s*[^<>]*?)\s*</p>`)
	content = rePEmph.ReplaceAllString(content, "\n$1\n")

	// Horizontal rule variants
	reTopHR := regexp.MustCompile(`(?m)^[ \t]*---[ \t]*$`)
	content = reTopHR.ReplaceAllString(content, `<hr/>`)
	rePHR := regexp.MustCompile(`(?is)<p>\s*---\s*</p>`)
	content = rePHR.ReplaceAllString(content, `<hr/>`)

	// Headings inside containers: <div>## H</div> -> <div><h2>H</h2></div>
	reInH3 := regexp.MustCompile(`(?is)>(\s*###\s+)(.+?)\s*<`)
	content = reInH3.ReplaceAllString(content, `><h3>$2</h3><`)
	reInH2 := regexp.MustCompile(`(?is)>(\s*##\s+)(.+?)\s*<`)
	content = reInH2.ReplaceAllString(content, `><h2>$2</h2><`)
	reInH1 := regexp.MustCompile(`(?is)>(\s*#\s+)(.+?)\s*<`)
	content = reInH1.ReplaceAllString(content, `><h1>$2</h1><`)

	return content
}

// 7) Normalize inline pipe tables that were collapsed into a single line
func normalizeInlinePipeTables(content string) string {
	return protectPreBlocks(content, func(s string) string {
		paraRe := regexp.MustCompile(`(?is)<p>([\s\S]*?\|[\s\S]*?)</p>`)
		s = paraRe.ReplaceAllStringFunc(s, func(p string) string {
			if strings.Count(p, "|") >= 8 || strings.Contains(p, "---") {
				return strings.ReplaceAll(p, "| |", "|\n|")
			}
			return p
		})
		if strings.Count(s, "| |") >= 2 {
			s = strings.ReplaceAll(s, "| |", "|\n|")
		}
		return s
	})
}

// 8) Convert ```lang fences to <pre><code class="language-...">...</code></pre>
func convertFences(s string) string {
	re := regexp.MustCompile("(?s)```([a-zA-Z0-9_-]*)\\s*(.*?)```")
	return re.ReplaceAllStringFunc(s, func(m string) string {
		sm := re.FindStringSubmatch(m)
		if len(sm) < 3 {
			return m
		}
		lang := strings.TrimSpace(sm[1])
		code := cleanStyleHeader(sm[2])
		return fmt.Sprintf(`<pre><code class="language-%s">%s</code></pre>`, lang, escapeCode(code))
	})
}

func escapeCode(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func cleanStyleHeader(code string) string {
	lines := strings.Split(code, "\n")
	if len(lines) == 0 {
		return code
	}
	first := strings.TrimSpace(lines[0])
	preLine := regexp.MustCompile(`^pre\s+code\s*\{[^}]*\}\s*$`)
	if preLine.MatchString(first) {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

// Merge multiple > lines into a single blockquote
func processBlockquotes(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	var in bool
	var buf []string

	flush := func() {
		if len(buf) > 0 {
			out = append(out, "<blockquote><p>"+strings.Join(buf, " ")+"</p></blockquote>")
			buf = nil
		}
	}

	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, ">") || strings.HasPrefix(t, "&gt;") {
			var text string
			if strings.HasPrefix(t, "&gt;") {
				text = strings.TrimSpace(t[4:])
			} else {
				text = strings.TrimSpace(t[1:])
			}
			in = true
			buf = append(buf, text)
			continue
		}
		if in {
			flush()
			in = false
		}
		out = append(out, ln)
	}
	if in {
		flush()
	}
	return strings.Join(out, "\n")
}
