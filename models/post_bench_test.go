package models

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// stripHTML
// ---------------------------------------------------------------------------

func BenchmarkStripHTML_Short(b *testing.B) {
	input := "<p>Hello <strong>world</strong></p>"
	for i := 0; i < b.N; i++ {
		stripHTML(input)
	}
}

func BenchmarkStripHTML_Medium(b *testing.B) {
	input := strings.Repeat(`<div class="post"><h2>Title</h2><p>Paragraph with <a href="#">link</a> and <em>emphasis</em>.</p></div>`, 10)
	for i := 0; i < b.N; i++ {
		stripHTML(input)
	}
}

func BenchmarkStripHTML_Long(b *testing.B) {
	input := strings.Repeat(`<div class="post"><h2>Title</h2><p>Paragraph with <a href="#">link</a> and <em>emphasis</em>.</p></div>`, 100)
	for i := 0; i < b.N; i++ {
		stripHTML(input)
	}
}

// ---------------------------------------------------------------------------
// formatPostDates
// ---------------------------------------------------------------------------

func BenchmarkFormatPostDates(b *testing.B) {
	now := time.Now().Format(time.RFC3339)
	earlier := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	for i := 0; i < b.N; i++ {
		post := Post{
			CreatedAt:       now,
			PublicationDate: earlier,
			LastEditDate:    now,
		}
		formatPostDates(&post)
	}
}

func BenchmarkFormatPostDates_SameDates(b *testing.B) {
	now := time.Now().Format(time.RFC3339)
	for i := 0; i < b.N; i++ {
		post := Post{
			CreatedAt:       now,
			PublicationDate: "",
			LastEditDate:    "",
		}
		formatPostDates(&post)
	}
}

// ---------------------------------------------------------------------------
// trimContent
// ---------------------------------------------------------------------------

var sampleMarkdown = `# Introduction

This is the first paragraph with some **bold** and *italic* text.

<more-->

## Second Section

Here is a code block:

` + "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```" + `

And more text after the code block with [a link](https://example.com).
`

func BenchmarkTrimContent_WithMoreTag(b *testing.B) {
	for i := 0; i < b.N; i++ {
		trimContent(sampleMarkdown)
	}
}

func BenchmarkTrimContent_NoMoreTag(b *testing.B) {
	content := strings.Repeat("This is a test paragraph with some words. ", 20)
	for i := 0; i < b.N; i++ {
		trimContent(content)
	}
}

func BenchmarkTrimContent_WithCodeBlocks(b *testing.B) {
	content := "Some text before.\n\n" + "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```" + "\n\nSome text after code.\n\n" + "```python\nprint('hello')\n```" + "\n\nFinal paragraph."
	for i := 0; i < b.N; i++ {
		trimContent(content)
	}
}

// ---------------------------------------------------------------------------
// previewContentRaw — full preview pipeline (includes markdown rendering)
// ---------------------------------------------------------------------------

func BenchmarkPreviewContentRaw_WithMoreTag(b *testing.B) {
	for i := 0; i < b.N; i++ {
		previewContentRaw(sampleMarkdown)
	}
}

func BenchmarkPreviewContentRaw_ShortContent(b *testing.B) {
	content := "A short paragraph."
	for i := 0; i < b.N; i++ {
		previewContentRaw(content)
	}
}

func BenchmarkPreviewContentRaw_LongContent(b *testing.B) {
	content := strings.Repeat("This is a test paragraph with some words for benchmarking. ", 50)
	for i := 0; i < b.N; i++ {
		previewContentRaw(content)
	}
}

// ---------------------------------------------------------------------------
// RenderContent — full markdown-to-HTML pipeline
// ---------------------------------------------------------------------------

func BenchmarkRenderContent_Short(b *testing.B) {
	content := "Hello **world**"
	for i := 0; i < b.N; i++ {
		RenderContent(content)
	}
}

func BenchmarkRenderContent_Medium(b *testing.B) {
	content := `# Heading

This is a **paragraph** with *emphasis* and [a link](https://example.com).

- Item 1
- Item 2
- Item 3

` + "```go\nfunc main() {}\n```"
	for i := 0; i < b.N; i++ {
		RenderContent(content)
	}
}

func BenchmarkRenderContent_Long(b *testing.B) {
	content := strings.Repeat(`# Heading

This is a **paragraph** with *emphasis* and [a link](https://example.com).

- Item 1
- Item 2
- Item 3

`, 10)
	for i := 0; i < b.N; i++ {
		RenderContent(content)
	}
}

// ---------------------------------------------------------------------------
// findContentBeforeMoreTag
// ---------------------------------------------------------------------------

func BenchmarkFindContentBeforeMoreTag_Found(b *testing.B) {
	content := "First paragraph.\n\n<more-->\n\nSecond paragraph."
	for i := 0; i < b.N; i++ {
		findContentBeforeMoreTag(content)
	}
}

func BenchmarkFindContentBeforeMoreTag_NotFound(b *testing.B) {
	content := strings.Repeat("No more tag here. Just regular text. ", 20)
	for i := 0; i < b.N; i++ {
		findContentBeforeMoreTag(content)
	}
}

// ---------------------------------------------------------------------------
// fenceRe (pre-compiled regex) — sanity check
// ---------------------------------------------------------------------------

func BenchmarkFenceRegex(b *testing.B) {
	content := "before\n```go\nfunc() {}\n```\nmiddle\n```python\nprint()\n```\nafter"
	for i := 0; i < b.N; i++ {
		fenceRe.ReplaceAllString(content, " ")
	}
}
