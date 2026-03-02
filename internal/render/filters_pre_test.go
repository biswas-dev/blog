package render

import (
	"strings"
	"testing"
)

func TestNormalizeWhitespaceAndBreaks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"nbsp to space", "hello\u00A0world", "hello world"},
		{"en space", "a\u2002b", "a b"},
		{"em space", "a\u2003b", "a b"},
		{"figure space", "a\u2007b", "a b"},
		{"narrow nbsp", "a\u202Fb", "a b"},
		{"html nbsp entity", "a&nbsp;b", "a b"},
		{"numeric nbsp entity", "a&#160;b", "a b"},
		{"crlf to lf", "a\r\nb", "a\nb"},
		{"br tag", "a<br>b", "a\nb"},
		{"br self-closing", "a<br/>b", "a\nb"},
		{"br spaced", "a<br />b", "a\nb"},
		{"no changes", "plain text", "plain text"},
		{"empty string", "", ""},
		{"multiple replacements", "a\u00A0b<br>c\r\nd", "a b\nc\nd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeWhitespaceAndBreaks(tt.input)
			if got != tt.want {
				t.Errorf("normalizeWhitespaceAndBreaks(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReplaceMoreTag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"standard more tag", "before<more-->after", "beforeafter"},
		{"spaced more tag", "before<more -->after", "beforeafter"},
		{"html entity more", "before&lt;more--&gt;after", "beforeafter"},
		{"html entity spaced", "before&lt;more --&gt;after", "beforeafter"},
		{"no more tag", "just text", "just text"},
		{"empty string", "", ""},
		{"more tag at start", "<more-->rest", "rest"},
		{"more tag at end", "text<more-->", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceMoreTag(tt.input)
			if got != tt.want {
				t.Errorf("replaceMoreTag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeCode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ampersand", "a & b", "a &amp; b"},
		{"less than", "a < b", "a &lt; b"},
		{"greater than", "a > b", "a &gt; b"},
		{"html tags", "<div>text</div>", "&lt;div&gt;text&lt;/div&gt;"},
		{"no escaping", "plain text", "plain text"},
		{"empty", "", ""},
		{"all special", "<&>", "&lt;&amp;&gt;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeCode(tt.input)
			if got != tt.want {
				t.Errorf("escapeCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanStyleHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes pre code CSS header",
			input: "pre code { font-size: 14px; }\nactual code\nmore code",
			want:  "actual code\nmore code",
		},
		{
			name:  "preserves code without CSS header",
			input: "actual code\nmore code",
			want:  "actual code\nmore code",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "single line code",
			input: "fmt.Println()",
			want:  "fmt.Println()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanStyleHeader(tt.input)
			if got != tt.want {
				t.Errorf("cleanStyleHeader(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcessBlockquotes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		checkFunc func(string) bool
		desc      string
	}{
		{
			name:      "single blockquote line",
			input:     "> This is quoted",
			checkFunc: func(s string) bool { return strings.Contains(s, "<blockquote>") },
			desc:      "should contain <blockquote> tag",
		},
		{
			name:      "multiple blockquote lines merged",
			input:     "> Line one\n> Line two",
			checkFunc: func(s string) bool { return strings.Contains(s, "Line one") && strings.Contains(s, "Line two") },
			desc:      "should contain both lines",
		},
		{
			name:      "html entity blockquote",
			input:     "&gt; Quoted text",
			checkFunc: func(s string) bool { return strings.Contains(s, "<blockquote>") },
			desc:      "should handle &gt; prefix",
		},
		{
			name:      "no blockquote",
			input:     "Normal text\nMore text",
			checkFunc: func(s string) bool { return !strings.Contains(s, "<blockquote>") },
			desc:      "should not add blockquote tags",
		},
		{
			name:      "empty string",
			input:     "",
			checkFunc: func(s string) bool { return s == "" },
			desc:      "should return empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processBlockquotes(tt.input)
			if !tt.checkFunc(got) {
				t.Errorf("processBlockquotes(%q) = %q, %s", tt.input, got, tt.desc)
			}
		})
	}
}

func TestConvertFences(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		checkFunc func(string) bool
		desc      string
	}{
		{
			name:      "go code fence",
			input:     "```go\nfmt.Println()\n```",
			checkFunc: func(s string) bool { return strings.Contains(s, `language-go`) },
			desc:      "should have language-go class",
		},
		{
			name:      "html escaping in fence",
			input:     "```html\n<div>test</div>\n```",
			checkFunc: func(s string) bool { return strings.Contains(s, "&lt;div&gt;") },
			desc:      "should escape HTML inside code",
		},
		{
			name:      "no language fence",
			input:     "```\ncode here\n```",
			checkFunc: func(s string) bool { return strings.Contains(s, "<pre><code") },
			desc:      "should still wrap in pre/code",
		},
		{
			name:      "no fences",
			input:     "just text",
			checkFunc: func(s string) bool { return s == "just text" },
			desc:      "should pass through unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertFences(tt.input)
			if !tt.checkFunc(got) {
				t.Errorf("convertFences(%q) = %q, %s", tt.input, got, tt.desc)
			}
		})
	}
}

func TestEnsureListSeparation(t *testing.T) {
	t.Run("adds blank line before list", func(t *testing.T) {
		input := "Some text\n- list item"
		got := ensureListSeparation(input)
		if !strings.Contains(got, "\n\n") {
			t.Errorf("expected blank line before list, got %q", got)
		}
	})

	t.Run("no change when already separated", func(t *testing.T) {
		input := "Some text\n\n- list item"
		got := ensureListSeparation(input)
		if got != input {
			t.Errorf("expected no change, got %q", got)
		}
	})

	t.Run("handles ordered list", func(t *testing.T) {
		input := "Some text\n1. first item"
		got := ensureListSeparation(input)
		if !strings.Contains(got, "\n\n") {
			t.Errorf("expected blank line before ordered list, got %q", got)
		}
	})
}

func TestUnwrapListLikeContainers(t *testing.T) {
	t.Run("unwraps div with list item", func(t *testing.T) {
		input := `<div style="font-size:14px">- Item one</div>`
		got := unwrapListLikeContainers(input)
		if !strings.Contains(got, "- Item one") {
			t.Errorf("expected unwrapped list item, got %q", got)
		}
		if strings.Contains(got, "<div") {
			t.Errorf("div should be removed, got %q", got)
		}
	})

	t.Run("unwraps p with ordered list item", func(t *testing.T) {
		input := `<p>1. First item</p>`
		got := unwrapListLikeContainers(input)
		if !strings.Contains(got, "1.") && !strings.Contains(got, "First item") {
			t.Errorf("expected unwrapped ordered item, got %q", got)
		}
	})

	t.Run("no change for normal div", func(t *testing.T) {
		input := `<div>Normal text here</div>`
		got := unwrapListLikeContainers(input)
		if got != input {
			t.Errorf("expected no change for non-list div, got %q", got)
		}
	})
}

func TestPreprocessLooseMarkdownHTML(t *testing.T) {
	t.Run("converts heading in paragraph", func(t *testing.T) {
		input := "<p>## My Heading</p>"
		got := preprocessLooseMarkdownHTML(input)
		if !strings.Contains(got, "<h2>My Heading</h2>") {
			t.Errorf("expected h2 tag, got %q", got)
		}
	})

	t.Run("converts h1 in paragraph", func(t *testing.T) {
		input := "<p># Title</p>"
		got := preprocessLooseMarkdownHTML(input)
		if !strings.Contains(got, "<h1>Title</h1>") {
			t.Errorf("expected h1 tag, got %q", got)
		}
	})

	t.Run("converts h3 in paragraph", func(t *testing.T) {
		input := "<p>### Subtitle</p>"
		got := preprocessLooseMarkdownHTML(input)
		if !strings.Contains(got, "<h3>Subtitle</h3>") {
			t.Errorf("expected h3 tag, got %q", got)
		}
	})

	t.Run("converts horizontal rule", func(t *testing.T) {
		input := "<p>---</p>"
		got := preprocessLooseMarkdownHTML(input)
		if !strings.Contains(got, "<hr/>") {
			t.Errorf("expected hr tag, got %q", got)
		}
	})

	t.Run("adds blank line after block containers", func(t *testing.T) {
		input := "</div>Next text"
		got := preprocessLooseMarkdownHTML(input)
		if !strings.Contains(got, "</div>\n\n") {
			t.Errorf("expected blank line after </div>, got %q", got)
		}
	})
}

func TestNormalizeInlinePipeTables(t *testing.T) {
	t.Run("no change for non-table content", func(t *testing.T) {
		input := "<p>Normal text</p>"
		got := normalizeInlinePipeTables(input)
		if got != input {
			t.Errorf("expected no change, got %q", got)
		}
	})

	t.Run("preserves pre blocks", func(t *testing.T) {
		input := "<pre>| col1 | col2 |</pre>"
		got := normalizeInlinePipeTables(input)
		if !strings.Contains(got, "<pre>") {
			t.Errorf("pre block should be preserved, got %q", got)
		}
	})
}

func TestStripStyleSnippets(t *testing.T) {
	t.Run("removes token CSS", func(t *testing.T) {
		input := ".token.keyword { color: red; }\nActual content"
		got := stripStyleSnippets(input)
		if strings.Contains(got, ".token.keyword") {
			t.Errorf("CSS should be stripped, got %q", got)
		}
		if !strings.Contains(got, "Actual content") {
			t.Errorf("content should be preserved, got %q", got)
		}
	})

	t.Run("preserves CSS in pre blocks", func(t *testing.T) {
		input := "<pre>.token.keyword { color: red; }</pre>"
		got := stripStyleSnippets(input)
		if !strings.Contains(got, ".token.keyword") {
			t.Errorf("CSS inside pre should be preserved, got %q", got)
		}
	})
}

// --- renderer.go helpers ---

func TestStrconvItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{999, "999"},
	}

	for _, tt := range tests {
		got := strconvItoa(tt.input)
		if got != tt.want {
			t.Errorf("strconvItoa(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPlaceholder(t *testing.T) {
	got := placeholder("PRE", 0)
	if got != "[[[PRE_BLOCK_0]]]" {
		t.Errorf("placeholder(PRE, 0) = %q, want [[[PRE_BLOCK_0]]]", got)
	}

	got2 := placeholder("EMPH", 5)
	if got2 != "[[[EMPH_BLOCK_5]]]" {
		t.Errorf("placeholder(EMPH, 5) = %q, want [[[EMPH_BLOCK_5]]]", got2)
	}
}

func TestProtectPreBlocks(t *testing.T) {
	t.Run("protects pre blocks during transformation", func(t *testing.T) {
		input := "before <pre>keep this</pre> after"
		got := protectPreBlocks(input, func(s string) string {
			return strings.ReplaceAll(s, "before", "BEFORE")
		})
		if !strings.Contains(got, "BEFORE") {
			t.Errorf("transform should run on non-pre content, got %q", got)
		}
		if !strings.Contains(got, "<pre>keep this</pre>") {
			t.Errorf("pre block should be preserved, got %q", got)
		}
	})

	t.Run("handles no pre blocks", func(t *testing.T) {
		input := "just text"
		got := protectPreBlocks(input, func(s string) string {
			return strings.ToUpper(s)
		})
		if got != "JUST TEXT" {
			t.Errorf("expected uppercase, got %q", got)
		}
	})
}
