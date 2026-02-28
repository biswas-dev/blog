package render

import (
	"regexp"
	"strings"

	"github.com/russross/blackfriday/v2"
)

// Options to toggle features without touching code.
type RendererOptions struct {
	AddListClasses       bool // add Tailwind classes to <ul>/<ol>/<li>
	AddBlockquoteClasses bool // style blockquotes
	EnableLightbox       bool // wrap images in anchors inside image galleries or standalone <img>
	EnableYouTubeEmbeds  bool // turn plain YT links into iframes
	EnableTaskListHTML   bool // turn - [x] into checkboxes
	EnableMermaid        bool // convert ```mermaid to <div class="mermaid">
	ProtectInlineCSS     bool // strip one-line CSS outsiders (pre code{...}) pasted accidentally
}

// Default sane options for the blog.
func DefaultOptions() RendererOptions {
	return RendererOptions{
		AddListClasses:       true,
		AddBlockquoteClasses: true,
		EnableLightbox:       true,
		EnableYouTubeEmbeds:  true,
		EnableTaskListHTML:   true,
		EnableMermaid:        true,
		ProtectInlineCSS:     true,
	}
}

type Renderer struct {
	Opt RendererOptions
}

func NewRenderer(opt RendererOptions) *Renderer {
	return &Renderer{Opt: opt}
}

// Render runs the full pipeline and returns final HTML.
func (r *Renderer) Render(content string) string {
	html, _ := r.RenderWithDebug(content, false)
	return html
}

// RenderWithDebug returns the final HTML and (optionally) every stage output for inspection.
func (r *Renderer) RenderWithDebug(content string, includeStages bool) (string, map[string]string) {
	stages := map[string]string{}

	stage := func(name, s string) string {
		if includeStages {
			stages[name] = s
		}
		return s
	}

	// --- PRE ---
	s := content
	s = stage("00_raw", s)
	s = normalizeWhitespaceAndBreaks(s)
	s = stage("01_normalized", s)

	if r.Opt.ProtectInlineCSS {
		s = stripStyleSnippets(s)
		s = stage("02_strip_style_snippets", s)
	}

	s = replaceMoreTag(s)
	s = stage("03_more_tag", s)

	s = unwrapListLikeContainers(s)    // <div>- item</div> -> "- item"
	s = ensureListSeparation(s)        // blank line before first -/1.
	s = preprocessLooseMarkdownHTML(s) // headings/quotes from HTML-wrapped lines, etc.
	s = normalizeInlinePipeTables(s)
	s = convertFences(s) // ```lang -> <pre><code class="language-lang">...</code></pre>
	s = stage("04_preprocessed", s)

	// --- MARKDOWN ---
	md := renderMarkdown(s)
	md = stage("05_markdown", md)

	// --- POST ---
	if r.Opt.EnableMermaid {
		md = transformMermaidBlocks(md)
		md = stage("06_mermaid", md)
	}
	if r.Opt.EnableTaskListHTML {
		md = taskListToHTML(md)
		md = stage("07_tasklist", md)
	}
	if r.Opt.EnableYouTubeEmbeds {
		md = embedYouTube(md)
		md = stage("08_youtube", md)
	}
	if r.Opt.AddListClasses {
		md = addListClasses(md)
		md = stage("09_list_classes", md)
	}
	if r.Opt.AddBlockquoteClasses {
		md = addBlockquoteClasses(md)
		md = stage("10_blockquote_classes", md)
	}
	md = convertInlineEmphasisInHTML(md)
	md = stage("11_inline_emphasis", md)

	if r.Opt.EnableLightbox {
		md = wrapImageGalleries(md)
		md = stage("12_lightbox", md)
	}

	final := md
	return final, stages
}

// ---- Markdown renderer ----

func renderMarkdown(content string) string {
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	exts := blackfriday.CommonExtensions |
		blackfriday.AutoHeadingIDs |
		blackfriday.FencedCode |
		blackfriday.Tables |
		blackfriday.Strikethrough
	renderer := blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{})
	out := blackfriday.Run([]byte(content), blackfriday.WithExtensions(exts), blackfriday.WithRenderer(renderer))
	return string(out)
}

// ---- Generic helpers shared by filters ----

// preBlockRe matches <pre>...</pre> blocks (case-insensitive, dotall).
var preBlockRe = regexp.MustCompile(`(?is)<pre[\s\S]*?</pre>`)

// Protects <pre> blocks with placeholders while f runs, then restores them.
func protectPreBlocks(s string, f func(string) string) string {
	var stash []string
	s = preBlockRe.ReplaceAllStringFunc(s, func(m string) string {
		stash = append(stash, m)
		return placeholder("PRE", len(stash)-1)
	})
	s = f(s)
	for i, m := range stash {
		s = strings.ReplaceAll(s, placeholder("PRE", i), m)
	}
	return s
}

func placeholder(tag string, i int) string {
	return "[[[" + tag + "_BLOCK_" + itoa(i) + "]]]"
}

func itoa(i int) string { return strconvItoa(i) }

// tiny local int->string (keeps imports tidy)
func strconvItoa(i int) string {
	// cheap conversion; avoids bringing in fmt just for this
	var digits [20]byte
	pos := len(digits)
	n := i
	if n == 0 {
		return "0"
	}
	for n > 0 {
		pos--
		digits[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[pos:])
}
