package utils

import "testing"

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"no tags", "hello world", "hello world"},
		{"simple tag", "<p>hello</p>", "hello"},
		{"nested tags", "<div><p>hello</p></div>", "hello"},
		{"self-closing tag", "hello<br/>world", "helloworld"},
		{"attributes", `<a href="http://example.com">link</a>`, "link"},
		{"mixed content", "before <b>bold</b> after", "before bold after"},
		{"unclosed tag", "hello <b>world", "hello world"},
		{"angle brackets in text", "1 < 2 > 0", "1  0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.input)
			if got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
