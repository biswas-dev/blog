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
}
