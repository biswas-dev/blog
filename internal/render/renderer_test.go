package render

import (
	"strings"
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if !opts.AddListClasses {
		t.Error("Expected AddListClasses to be true")
	}
	if !opts.AddBlockquoteClasses {
		t.Error("Expected AddBlockquoteClasses to be true")
	}
	if !opts.EnableLightbox {
		t.Error("Expected EnableLightbox to be true")
	}
	if !opts.EnableYouTubeEmbeds {
		t.Error("Expected EnableYouTubeEmbeds to be true")
	}
	if !opts.EnableTaskListHTML {
		t.Error("Expected EnableTaskListHTML to be true")
	}
	if !opts.EnableMermaid {
		t.Error("Expected EnableMermaid to be true")
	}
	if !opts.ProtectInlineCSS {
		t.Error("Expected ProtectInlineCSS to be true")
	}
}

func TestNewRenderer(t *testing.T) {
	opts := RendererOptions{
		AddListClasses: true,
		EnableLightbox: false,
	}

	r := NewRenderer(opts)

	if r == nil {
		t.Fatal("NewRenderer returned nil")
	}

	if r.Opt.AddListClasses != true {
		t.Error("Expected AddListClasses to be true")
	}

	if r.Opt.EnableLightbox != false {
		t.Error("Expected EnableLightbox to be false")
	}
}

func TestRender(t *testing.T) {
	r := NewRenderer(DefaultOptions())

	t.Run("renders basic markdown", func(t *testing.T) {
		input := "# Hello World\n\nThis is a paragraph."
		output := r.Render(input)

		if !strings.Contains(output, "<h1") {
			t.Error("Expected h1 tag in output")
		}
		if !strings.Contains(output, "Hello World") {
			t.Error("Expected 'Hello World' in output")
		}
		if !strings.Contains(output, "<p>") {
			t.Error("Expected p tag in output")
		}
		if !strings.Contains(output, "This is a paragraph") {
			t.Error("Expected paragraph text in output")
		}
	})

	t.Run("renders bold text", func(t *testing.T) {
		input := "This is **bold** text."
		output := r.Render(input)

		if !strings.Contains(output, "<strong>bold</strong>") {
			t.Error("Expected <strong> tag for bold text")
		}
	})

	t.Run("renders italic text", func(t *testing.T) {
		input := "This is *italic* text."
		output := r.Render(input)

		if !strings.Contains(output, "<em>italic</em>") {
			t.Error("Expected <em> tag for italic text")
		}
	})

	t.Run("renders links", func(t *testing.T) {
		input := "[Link Text](https://example.com)"
		output := r.Render(input)

		if !strings.Contains(output, `<a href="https://example.com"`) {
			t.Error("Expected anchor tag with href")
		}
		if !strings.Contains(output, "Link Text") {
			t.Error("Expected link text in output")
		}
	})

	t.Run("renders code blocks", func(t *testing.T) {
		input := "```go\nfunc main() {}\n```"
		output := r.Render(input)

		if !strings.Contains(output, "<pre>") {
			t.Error("Expected <pre> tag for code block")
		}
		if !strings.Contains(output, "<code") {
			t.Error("Expected <code> tag for code block")
		}
		if !strings.Contains(output, "func main()") {
			t.Error("Expected code content in output")
		}
	})

	t.Run("renders inline code", func(t *testing.T) {
		input := "Use the `fmt.Println` function."
		output := r.Render(input)

		if !strings.Contains(output, "<code>") {
			t.Error("Expected <code> tag for inline code")
		}
		if !strings.Contains(output, "fmt.Println") {
			t.Error("Expected code content in output")
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		output := r.Render("")

		if output == "" {
			// Empty input produces empty or minimal output
			return
		}
		// Some renderers might add wrapping tags
		if len(output) > 50 {
			t.Errorf("Expected minimal output for empty input, got: %s", output)
		}
	})

	t.Run("handles whitespace", func(t *testing.T) {
		input := "   \n\n   \n"
		output := r.Render(input)

		// Should handle whitespace gracefully
		if len(output) > 100 {
			t.Errorf("Expected minimal output for whitespace, got length: %d", len(output))
		}
	})
}

func TestRenderWithDebug(t *testing.T) {
	r := NewRenderer(DefaultOptions())

	t.Run("returns HTML and stages with debug enabled", func(t *testing.T) {
		input := "# Test"
		html, stages := r.RenderWithDebug(input, true)

		if html == "" {
			t.Error("Expected non-empty HTML output")
		}

		if !strings.Contains(html, "<h1") {
			t.Error("Expected h1 tag in HTML output")
		}

		if len(stages) == 0 {
			t.Error("Expected non-empty stages map when debug is enabled")
		}

		// Check that some expected stages exist
		if _, exists := stages["00_raw"]; !exists {
			t.Error("Expected '00_raw' stage")
		}
	})

	t.Run("returns HTML without stages when debug disabled", func(t *testing.T) {
		input := "# Test"
		html, stages := r.RenderWithDebug(input, false)

		if html == "" {
			t.Error("Expected non-empty HTML output")
		}

		if len(stages) != 0 {
			t.Error("Expected empty stages map when debug is disabled")
		}
	})
}

func TestRendererOptions(t *testing.T) {
	t.Run("renders with AddListClasses disabled", func(t *testing.T) {
		opts := DefaultOptions()
		opts.AddListClasses = false
		r := NewRenderer(opts)

		input := "- Item 1\n- Item 2"
		output := r.Render(input)

		if !strings.Contains(output, "<ul>") {
			t.Error("Expected <ul> tag")
		}
		if !strings.Contains(output, "<li>") {
			t.Error("Expected <li> tag")
		}
	})

	t.Run("renders blockquotes", func(t *testing.T) {
		r := NewRenderer(DefaultOptions())

		input := "> This is a quote"
		output := r.Render(input)

		if !strings.Contains(output, "<blockquote>") {
			t.Error("Expected <blockquote> tag")
		}
		if !strings.Contains(output, "This is a quote") {
			t.Error("Expected quote text in output")
		}
	})

	t.Run("renders ordered lists", func(t *testing.T) {
		r := NewRenderer(DefaultOptions())

		input := "1. First\n2. Second\n3. Third"
		output := r.Render(input)

		if !strings.Contains(output, "<ol>") {
			t.Error("Expected <ol> tag")
		}
		if !strings.Contains(output, "First") {
			t.Error("Expected list item text")
		}
	})

	t.Run("renders unordered lists", func(t *testing.T) {
		r := NewRenderer(DefaultOptions())

		input := "- Apple\n- Banana\n- Cherry"
		output := r.Render(input)

		if !strings.Contains(output, "<ul>") {
			t.Error("Expected <ul> tag")
		}
		if !strings.Contains(output, "Apple") {
			t.Error("Expected list item text")
		}
	})

	t.Run("renders headings at different levels", func(t *testing.T) {
		r := NewRenderer(DefaultOptions())

		tests := []struct {
			input string
			tag   string
		}{
			{"# H1", "<h1"},
			{"## H2", "<h2"},
			{"### H3", "<h3"},
			{"#### H4", "<h4"},
			{"##### H5", "<h5"},
			{"###### H6", "<h6"},
		}

		for _, tt := range tests {
			output := r.Render(tt.input)
			if !strings.Contains(output, tt.tag) {
				t.Errorf("Expected %s tag for input %q", tt.tag, tt.input)
			}
		}
	})
}

func TestMoreTagReplacement(t *testing.T) {
	r := NewRenderer(DefaultOptions())

	t.Run("removes <more--> tag", func(t *testing.T) {
		input := "First paragraph\n\n<more-->\n\nSecond paragraph"
		output := r.Render(input)

		// The <more--> tag should be removed or replaced
		if strings.Contains(output, "<more-->") {
			t.Error("Expected <more--> tag to be removed")
		}

		// Both paragraphs should still be present
		if !strings.Contains(output, "First paragraph") {
			t.Error("Expected first paragraph in output")
		}
		if !strings.Contains(output, "Second paragraph") {
			t.Error("Expected second paragraph in output")
		}
	})

	t.Run("handles content without more tag", func(t *testing.T) {
		input := "Just one paragraph"
		output := r.Render(input)

		if !strings.Contains(output, "Just one paragraph") {
			t.Error("Expected paragraph text in output")
		}
	})
}

func TestComplexMarkdown(t *testing.T) {
	r := NewRenderer(DefaultOptions())

	t.Run("renders nested lists", func(t *testing.T) {
		input := `- Item 1
  - Nested 1
  - Nested 2
- Item 2`
		output := r.Render(input)

		if !strings.Contains(output, "<ul>") {
			t.Error("Expected <ul> tag")
		}
		if !strings.Contains(output, "Item 1") {
			t.Error("Expected list items in output")
		}
	})

	t.Run("renders tables", func(t *testing.T) {
		input := `| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |`
		output := r.Render(input)

		if !strings.Contains(output, "<table>") {
			t.Error("Expected <table> tag")
		}
		if !strings.Contains(output, "Header 1") {
			t.Error("Expected table headers in output")
		}
	})

	t.Run("renders mixed content", func(t *testing.T) {
		input := `# Title

This is **bold** and *italic*.

- List item
- Another item

` + "```" + `
code block
` + "```"
		output := r.Render(input)

		if !strings.Contains(output, "<h1") {
			t.Error("Expected heading")
		}
		if !strings.Contains(output, "<strong>") {
			t.Error("Expected bold text")
		}
		if !strings.Contains(output, "<em>") {
			t.Error("Expected italic text")
		}
		if !strings.Contains(output, "<ul>") {
			t.Error("Expected list")
		}
		if !strings.Contains(output, "<pre>") {
			t.Error("Expected code block")
		}
	})
}

func BenchmarkRender(b *testing.B) {
	r := NewRenderer(DefaultOptions())
	input := `# Heading

This is a **paragraph** with *emphasis*.

- Item 1
- Item 2

` + "```go\nfunc main() {}\n```"

	for i := 0; i < b.N; i++ {
		r.Render(input)
	}
}

func BenchmarkRenderSimple(b *testing.B) {
	r := NewRenderer(DefaultOptions())
	input := "This is a simple paragraph."

	for i := 0; i < b.N; i++ {
		r.Render(input)
	}
}

func BenchmarkRenderComplex(b *testing.B) {
	r := NewRenderer(DefaultOptions())
	input := strings.Repeat(`# Heading

This is a **paragraph** with *emphasis* and [a link](https://example.com).

- Item 1
- Item 2
- Item 3

`, 10)

	for i := 0; i < b.N; i++ {
		r.Render(input)
	}
}
