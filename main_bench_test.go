package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// xmlEscape — used in RSS feed generation
// ---------------------------------------------------------------------------

func BenchmarkXmlEscape_Clean(b *testing.B) {
	input := "A simple title without special characters"
	for i := 0; i < b.N; i++ {
		xmlEscape(input)
	}
}

func BenchmarkXmlEscape_Special(b *testing.B) {
	input := `Title with "quotes" & <brackets> and 'apostrophes'`
	for i := 0; i < b.N; i++ {
		xmlEscape(input)
	}
}

func BenchmarkXmlEscape_Long(b *testing.B) {
	input := strings.Repeat(`Content with & and < and > special "chars"`, 50)
	for i := 0; i < b.N; i++ {
		xmlEscape(input)
	}
}
