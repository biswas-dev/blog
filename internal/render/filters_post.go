package render

import (
	"fmt"
	"regexp"
	"strings"
)

// 1) Add classes to UL/OL/LI (Tailwind-friendly)
func addListClasses(content string) string {
	ulRe := regexp.MustCompile(`(?i)<ul\b([^>]*)>`)
	olRe := regexp.MustCompile(`(?i)<ol\b([^>]*)>`)
	liRe := regexp.MustCompile(`(?i)<li\b([^>]*)>`)

	withClass := func(tag, classes, attrs string) string {
		if strings.Contains(strings.ToLower(attrs), "class=") {
			return "<" + tag + attrs + ">"
		}
		return "<" + tag + ` class="` + classes + `"` + attrs + ">"
	}

	content = ulRe.ReplaceAllStringFunc(content, func(m string) string {
		attrs := ulRe.FindStringSubmatch(m)[1]
		return withClass("ul", "list-disc pl-2", attrs)
	})
	content = olRe.ReplaceAllStringFunc(content, func(m string) string {
		attrs := olRe.FindStringSubmatch(m)[1]
		return withClass("ol", "list-decimal pl-2", attrs)
	})
	content = liRe.ReplaceAllStringFunc(content, func(m string) string {
		attrs := liRe.FindStringSubmatch(m)[1]
		return withClass("li", "mb-2", attrs)
	})
	return content
}

// 2) Blockquote classes
func addBlockquoteClasses(content string) string {
	bqRe := regexp.MustCompile(`(?i)<blockquote\b([^>]*)>`)
	return bqRe.ReplaceAllStringFunc(content, func(m string) string {
		attrs := bqRe.FindStringSubmatch(m)[1]
		if strings.Contains(strings.ToLower(attrs), "class=") {
			return "<blockquote" + attrs + ">"
		}
		return `<blockquote class="p-4 my-4 border-s-4 border-gray-300 bg-gray-50 dark:border-gray-500 dark:bg-gray-800"` + attrs + ">"
	})
}

// 3) Convert "- [x] Task" into disabled checkboxes
func taskListToHTML(html string) string {
	// Only transform inside <li>...</li>
	re := regexp.MustCompile(`(?is)<li([^>]*)>\s*\[([ xX])\]\s*(.*?)</li>`)
	return re.ReplaceAllStringFunc(html, func(m string) string {
		sub := re.FindStringSubmatch(m)
		attrs := sub[1]
		checked := strings.EqualFold(sub[2], "x")
		text := sub[3]
		chk := `<input type="checkbox" disabled` + ternary(checked, " checked", "") + ` class="mr-2 align-middle">`
		if strings.Contains(strings.ToLower(attrs), "class=") {
			return `<li` + attrs + `>` + chk + text + `</li>`
		}
		return `<li class="task-item"` + attrs + `>` + chk + text + `</li>`
	})
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// 4) Replace paragraph-wrapped YouTube links with responsive iframes
func embedYouTube(html string) string {
	re := regexp.MustCompile(`(?is)<p>\s*<a[^>]+href="(https?://(?:www\.)?(?:youtube\.com/watch\?v=|youtu\.be/)([^"&<>\s]+)[^"]*)"[^>]*>[^<]*</a>\s*</p>`)
	return re.ReplaceAllStringFunc(html, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		id := sub[2]
		iframe := `<div class="aspect-video w-full max-w-3xl"><iframe src="https://www.youtube.com/embed/` + id + `" title="YouTube video" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen style="width:100%;height:100%"></iframe></div>`
		return iframe
	})
}

// 5) Convert Prism-style mermaid code blocks to <div class="mermaid">
func transformMermaidBlocks(html string) string {
	re := regexp.MustCompile(`(?is)<pre><code class="language-mermaid">([\s\S]*?)</code></pre>`)
	return re.ReplaceAllString(html, `<div class="mermaid">$1</div>`)
}

// 6) Emphasis conversion inside already-rendered HTML text nodes
func convertInlineEmphasisInHTML(html string) string {
	codeRe := regexp.MustCompile("(?is)(<pre[\\s\\S]*?</pre>|<code[\\s\\S]*?</code>)")
	var stash []string
	html = codeRe.ReplaceAllStringFunc(html, func(m string) string {
		stash = append(stash, m)
		return placeholder("EMPH", len(stash)-1)
	})

	textNodeRe := regexp.MustCompile(">([^<]+)<")
	html = textNodeRe.ReplaceAllStringFunc(html, func(seg string) string {
		inner := seg[1 : len(seg)-1]
		// Skip processing if this looks like a placeholder
		if strings.Contains(inner, "[[[") && strings.Contains(inner, "]]]") {
			return seg
		}
		inner = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(inner, "<strong>$1</strong>")
		inner = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(inner, "<strong>$1</strong>")
		inner = regexp.MustCompile(`(^|[^*])\*([^*]+)\*([^*]|$)`).ReplaceAllString(inner, "$1<em>$2</em>$3")
		inner = regexp.MustCompile(`(^|[^_])_([^_]+)_([^_]|$)`).ReplaceAllString(inner, "$1<em>$2</em>$3")
		return ">" + inner + "<"
	})

	for i, m := range stash {
		html = strings.ReplaceAll(html, placeholder("EMPH", i), m)
	}
	return html
}

// 7) Image gallery + bare <img> -> lightbox links (keeps original <img> intact)
func wrapImageGalleries(html string) string {
	// Wrap any <img> inside .image-gallery; also allow standalone <img> paragraphs
	galleryRe := regexp.MustCompile(`(?is)(<div[^>]*class="[^"]*image-gallery[^"]*"[^>]*>)([\s\S]*?)(</div>)`)
	imgRe := regexp.MustCompile(`(?is)<img([^>]*?)\s+src="([^"]+)"([^>]*)>`)
	altRe := regexp.MustCompile(`(?i)alt="([^"]*)"`)
	existingLightboxRe := regexp.MustCompile(`(?is)<a[^>]*data-lightbox[^>]*>\s*<img[^>]*>\s*</a>`)

	transform := func(content string) string {
		// First, find and protect any existing lightbox-wrapped images
		if existingLightboxRe.MatchString(content) {
			// Content already has lightbox links, don't add more
			return content
		}

		// Only add lightbox wrapping if images are bare (not already wrapped)
		return imgRe.ReplaceAllStringFunc(content, func(imgTag string) string {
			m := imgRe.FindStringSubmatch(imgTag)
			if len(m) != 4 {
				return imgTag
			}
			preAttrs := m[1]
			src := m[2]
			postAttrs := m[3]
			attrs := preAttrs + " " + postAttrs
			alt := ""
			if am := altRe.FindStringSubmatch(attrs); len(am) == 2 {
				alt = am[1]
			}
			return fmt.Sprintf(
				`<a href="%s" data-lightbox="article-images" rel="lightbox[article-images]" data-title="%s"><img%s src="%s"%s></a>`,
				src, htmlEscapeAttr(alt), preAttrs, src, postAttrs,
			)
		})
	}

	// 7a) <figure> blocks: wrap <img> with lightbox using <figcaption> as data-title
	figureRe := regexp.MustCompile(`(?is)(<figure[^>]*>)([\s\S]*?)(</figure>)`)
	captionRe := regexp.MustCompile(`(?is)<figcaption[^>]*>([\s\S]*?)</figcaption>`)
	html = figureRe.ReplaceAllStringFunc(html, func(block string) string {
		// Skip if already has lightbox links
		if existingLightboxRe.MatchString(block) {
			return block
		}
		parts := figureRe.FindStringSubmatch(block)
		if len(parts) != 4 {
			return block
		}
		openTag, inner, closeTag := parts[1], parts[2], parts[3]

		// Extract figcaption text for lightbox title
		caption := ""
		if cm := captionRe.FindStringSubmatch(inner); len(cm) == 2 {
			// Strip HTML tags from caption for data-title
			caption = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(cm[1], "")
			caption = strings.TrimSpace(caption)
		}

		// Wrap bare <img> tags inside the figure with lightbox links
		inner = imgRe.ReplaceAllStringFunc(inner, func(imgTag string) string {
			m := imgRe.FindStringSubmatch(imgTag)
			if len(m) != 4 {
				return imgTag
			}
			preAttrs := m[1]
			src := m[2]
			postAttrs := m[3]
			// Use figcaption if available, fall back to alt text
			title := caption
			if title == "" {
				attrs := preAttrs + " " + postAttrs
				if am := altRe.FindStringSubmatch(attrs); len(am) == 2 {
					title = am[1]
				}
			}
			return fmt.Sprintf(
				`<a href="%s" data-lightbox="article-images" rel="lightbox[article-images]" data-title="%s"><img%s src="%s"%s></a>`,
				src, htmlEscapeAttr(title), preAttrs, src, postAttrs,
			)
		})

		return openTag + inner + closeTag
	})

	// 7b) Inside image-gallery containers
	html = galleryRe.ReplaceAllStringFunc(html, func(block string) string {
		parts := galleryRe.FindStringSubmatch(block)
		if len(parts) != 4 {
			return block
		}
		return parts[1] + transform(parts[2]) + parts[3]
	})

	// 7c) Standalone <p><img ...></p> -> lightbox (skip if already has lightbox links)
	imgPara := regexp.MustCompile(`(?is)<p>\s*(<img[^>]+>)\s*</p>`)
	existingParaLightboxRe := regexp.MustCompile(`(?is)<p>\s*<a[^>]*data-lightbox[^>]*>\s*<img[^>]*>\s*</a>\s*</p>`)

	html = imgPara.ReplaceAllStringFunc(html, func(m string) string {
		// Skip if this paragraph already has a lightbox link
		if existingParaLightboxRe.MatchString(m) {
			return m
		}

		im := imgRe.FindStringSubmatch(m)
		if len(im) != 4 {
			return m
		}
		src := im[2]
		preAttrs := im[1]
		postAttrs := im[3]
		attrs := preAttrs + " " + postAttrs
		alt := ""
		if am := altRe.FindStringSubmatch(attrs); len(am) == 2 {
			alt = am[1]
		}
		return fmt.Sprintf(
			`<p><a href="%s" data-lightbox="article-images" rel="lightbox[article-images]" data-title="%s"><img%s src="%s"%s></a></p>`,
			src, htmlEscapeAttr(alt), preAttrs, src, postAttrs,
		)
	})

	return html
}

func htmlEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `<`, `&lt;`)
	s = strings.ReplaceAll(s, `>`, `&gt;`)
	return s
}
