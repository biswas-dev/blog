package render

import (
	"fmt"
	"regexp"
	"strings"
)

// Package-level compiled regexps (compiled once at startup).
var (
	// stripStyleSnippets
	reStripPreCodeCSS  = regexp.MustCompile(`(?m)^\s*pre\s*code\s*\{[^}]*\}\s*$`)
	reStripTokenCSS    = regexp.MustCompile(`(?m)^\s*\.[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*\s*\{[^}]*\}\s*$`)

	// unwrapListLikeContainers
	reUnwrapListDiv        = regexp.MustCompile(`(?is)<div\b[^>]*>\s*([\-\*\+]\s*|\d+\.\s*)([\s\S]*?)</div>`)
	reUnwrapListP          = regexp.MustCompile(`(?is)<p\b[^>]*>\s*([\-\*\+]\s*|\d+\.\s*)([\s\S]*?)</p>`)
	reMergeConsecutiveList = regexp.MustCompile(`(?m)\n([-\*\+] |\d+\. ).+\n(?:(?:[-\*\+] |\d+\. ).+\n)+`)

	// ensureListSeparation
	reListSeparation = regexp.MustCompile(`(?m)([^\n])\n([ \t]*)([-*+]|\d+\.)\s+`)

	// preprocessLooseMarkdownHTML
	reCloseBlock       = regexp.MustCompile(`(?is)</(div|figure|section|table|blockquote|p)>\s*`)
	reTopLevelH3       = regexp.MustCompile(`(?m)^[ \t]*###[ \t]+(.+)$`)
	reTopLevelH2       = regexp.MustCompile(`(?m)^[ \t]*##[ \t]+(.+)$`)
	reTopLevelH1       = regexp.MustCompile(`(?m)^[ \t]*#[ \t]+(.+)$`)
	reParaH3           = regexp.MustCompile(`(?is)<p[^>]*>\s*###\s+(.+?)\s*</p>`)
	reParaH2           = regexp.MustCompile(`(?is)<p[^>]*>\s*##\s+(.+?)\s*</p>`)
	reParaH1           = regexp.MustCompile(`(?is)<p[^>]*>\s*#\s+(.+?)\s*</p>`)
	reParaUL           = regexp.MustCompile(`(?is)<p[^>]*>\s*([ \t]*)([-*+])\s+(.+?)\s*</p>`)
	reDivUL            = regexp.MustCompile(`(?is)<div[^>]*>\s*([ \t]*)([-*+])\s+(.+?)\s*</div>`)
	reParaOL           = regexp.MustCompile(`(?is)<p[^>]*>\s*([ \t]*)(\d+)\.\s+(.+?)\s*</p>`)
	reDivOL            = regexp.MustCompile(`(?is)<div[^>]*>\s*([ \t]*)(\d+)\.\s+(.+?)\s*</div>`)
	reIndent2UL        = regexp.MustCompile(`(?m)^[ ]{2}([-*+]\s+)`)
	reIndent2OL        = regexp.MustCompile(`(?m)^[ ]{2}(\d+\.\s+)`)
	reParaEmphasis     = regexp.MustCompile(`(?is)<p[^>]*>\s*([^<>]*?(\*\*.+?\*\*|__.+?__|\*[^*]+?\*|_[^_]+?_)\s*[^<>]*?)\s*</p>`)
	reTopLevelHR       = regexp.MustCompile(`(?m)^[ \t]*---[ \t]*$`)
	reParaHR           = regexp.MustCompile(`(?is)<p>\s*---\s*</p>`)
	reInnerH3          = regexp.MustCompile(`(?is)>(\s*###\s+)(.+?)\s*<`)
	reInnerH2          = regexp.MustCompile(`(?is)>(\s*##\s+)(.+?)\s*<`)
	reInnerH1          = regexp.MustCompile(`(?is)>(\s*#\s+)(.+?)\s*<`)

	// normalizeInlinePipeTables
	rePipeTablePara = regexp.MustCompile(`(?is)<p>([\s\S]*?\|[\s\S]*?)</p>`)

	// convertFences
	reCodeFence = regexp.MustCompile("(?s)```([a-zA-Z0-9_-]*)\\s*(.*?)```")

	// cleanStyleHeader
	reCleanStylePreCode = regexp.MustCompile(`^pre\s+code\s*\{[^}]*\}\s*$`)
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
		s = reStripPreCodeCSS.ReplaceAllString(s, "")
		s = reStripTokenCSS.ReplaceAllString(s, "")
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

	s := reUnwrapListDiv.ReplaceAllStringFunc(content, func(m string) string {
		sub := reUnwrapListDiv.FindStringSubmatch(m)
		return replaceFunc(sub)
	})

	s = reUnwrapListP.ReplaceAllStringFunc(s, func(m string) string {
		sub := reUnwrapListP.FindStringSubmatch(m)
		return replaceFunc(sub)
	})

	// Merge consecutive unwrapped items nicely and ensure a blank line at end.
	s = reMergeConsecutiveList.ReplaceAllStringFunc(s, func(block string) string {
		return strings.TrimRight(block, "\n") + "\n\n"
	})
	return s
}

// 5) Ensure there is a blank line before any list that follows text.
func ensureListSeparation(content string) string {
	return reListSeparation.ReplaceAllString(content, "$1\n\n$2$3 ")
}

//  6. Convert headings/quotes that appear inside plain HTML containers and
//     add blank lines after block containers so markdown resumes cleanly.
func preprocessLooseMarkdownHTML(content string) string {
	// Blank line after common block containers
	content = reCloseBlock.ReplaceAllString(content, "</$1>\n\n")

	// Convert raw top-level markdown headings (if WYSIWYG collapsed them)
	content = reTopLevelH3.ReplaceAllString(content, `<h3>$1</h3>`)
	content = reTopLevelH2.ReplaceAllString(content, `<h2>$1</h2>`)
	content = reTopLevelH1.ReplaceAllString(content, `<h1>$1</h1>`)

	// Multi-line blockquotes to a single <blockquote><p>...</p></blockquote>
	content = processBlockquotes(content)

	// Paragraph-wrapped headings
	content = reParaH3.ReplaceAllString(content, `<h3>$1</h3>`)
	content = reParaH2.ReplaceAllString(content, `<h2>$1</h2>`)
	content = reParaH1.ReplaceAllString(content, `<h1>$1</h1>`)

	// Paragraph/div wrapped list markers -> plain lines
	content = reParaUL.ReplaceAllString(content, "\n$1$2 $3\n")
	content = reDivUL.ReplaceAllString(content, "\n$1$2 $3\n")

	content = reParaOL.ReplaceAllString(content, "\n$1$2. $3\n")
	content = reDivOL.ReplaceAllString(content, "\n$1$2. $3\n")

	// Indent tweaks: normalize 2-space indented nested items to 4 spaces
	content = reIndent2UL.ReplaceAllString(content, `    $1`)
	content = reIndent2OL.ReplaceAllString(content, `    $1`)

	// Unwrap paragraphs that contain Markdown emphasis to let the renderer parse them
	content = reParaEmphasis.ReplaceAllString(content, "\n$1\n")

	// Horizontal rule variants
	content = reTopLevelHR.ReplaceAllString(content, `<hr/>`)
	content = reParaHR.ReplaceAllString(content, `<hr/>`)

	// Headings inside containers: <div>## H</div> -> <div><h2>H</h2></div>
	content = reInnerH3.ReplaceAllString(content, `><h3>$2</h3><`)
	content = reInnerH2.ReplaceAllString(content, `><h2>$2</h2><`)
	content = reInnerH1.ReplaceAllString(content, `><h1>$2</h1><`)

	return content
}

// 7) Normalize inline pipe tables that were collapsed into a single line
func normalizeInlinePipeTables(content string) string {
	return protectPreBlocks(content, func(s string) string {
		s = rePipeTablePara.ReplaceAllStringFunc(s, func(p string) string {
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
	return reCodeFence.ReplaceAllStringFunc(s, func(m string) string {
		sm := reCodeFence.FindStringSubmatch(m)
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
	if reCleanStylePreCode.MatchString(first) {
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
