package render

import (
	"strings"
	"testing"
)

func TestExtractFigcaption(t *testing.T) {
	tests := []struct {
		name  string
		inner string
		want  string
	}{
		{
			name:  "no figcaption",
			inner: `<img src="test.jpg" alt="test">`,
			want:  "",
		},
		{
			name:  "simple figcaption",
			inner: `<img src="test.jpg"><figcaption>My Caption</figcaption>`,
			want:  "My Caption",
		},
		{
			name:  "figcaption with HTML tags",
			inner: `<img src="test.jpg"><figcaption><em>Bold</em> caption</figcaption>`,
			want:  "Bold caption",
		},
		{
			name:  "figcaption with whitespace",
			inner: `<img src="test.jpg"><figcaption>  spaced  </figcaption>`,
			want:  "spaced",
		},
		{
			name:  "empty figcaption",
			inner: `<img src="test.jpg"><figcaption></figcaption>`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFigcaption(tt.inner)
			if got != tt.want {
				t.Errorf("extractFigcaption() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildLightboxImgTag(t *testing.T) {
	tests := []struct {
		name    string
		imgTag  string
		caption string
		checks  []string // substrings that must appear in output
	}{
		{
			name:    "wraps img with lightbox link",
			imgTag:  `<img class="photo" src="https://example.com/img.jpg" alt="A photo">`,
			caption: "My Caption",
			checks: []string{
				`<a href="https://example.com/img.jpg"`,
				`data-lightbox="article-images"`,
				`data-title="My Caption"`,
				`src="https://example.com/img.jpg"`,
			},
		},
		{
			name:    "falls back to alt when no caption",
			imgTag:  `<img src="https://example.com/img.jpg" alt="Alt Text">`,
			caption: "",
			checks: []string{
				`data-title="Alt Text"`,
				`<a href="https://example.com/img.jpg"`,
			},
		},
		{
			name:    "empty title when no caption or alt",
			imgTag:  `<img src="https://example.com/img.jpg">`,
			caption: "",
			checks: []string{
				`data-title=""`,
			},
		},
		{
			name:    "returns unchanged for non-matching tag",
			imgTag:  `<div>not an img</div>`,
			caption: "test",
			checks:  []string{`<div>not an img</div>`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLightboxImgTag(tt.imgTag, tt.caption)
			for _, check := range tt.checks {
				if !strings.Contains(got, check) {
					t.Errorf("buildLightboxImgTag() missing %q in output: %s", check, got)
				}
			}
		})
	}
}

func TestWrapFigureLightboxes(t *testing.T) {
	tests := []struct {
		name   string
		html   string
		checks []string
		absent []string
	}{
		{
			name:   "wraps bare img in figure",
			html:   `<figure><img src="https://example.com/photo.jpg" alt="Photo"></figure>`,
			checks: []string{`data-lightbox="article-images"`, `<a href="https://example.com/photo.jpg"`},
		},
		{
			name:   "skips already wrapped img",
			html:   `<figure><a data-lightbox="gallery"><img src="test.jpg"></a></figure>`,
			checks: []string{`<figure><a data-lightbox="gallery"><img src="test.jpg"></a></figure>`},
		},
		{
			name:   "no figures means no change",
			html:   `<p>Just a paragraph</p>`,
			checks: []string{`<p>Just a paragraph</p>`},
		},
		{
			name:   "uses figcaption as title",
			html:   `<figure><img src="https://example.com/photo.jpg"><figcaption>Caption Text</figcaption></figure>`,
			checks: []string{`data-title="Caption Text"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapFigureLightboxes(tt.html)
			for _, check := range tt.checks {
				if !strings.Contains(got, check) {
					t.Errorf("wrapFigureLightboxes() missing %q in:\n%s", check, got)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(got, absent) {
					t.Errorf("wrapFigureLightboxes() should not contain %q in:\n%s", absent, got)
				}
			}
		})
	}
}

func TestWrapGalleryContainers(t *testing.T) {
	t.Run("wraps bare img in gallery container", func(t *testing.T) {
		html := `<div class="image-gallery"><img src="https://example.com/a.jpg" alt="A"></div>`
		got := wrapGalleryContainers(html)
		if !strings.Contains(got, `data-lightbox=`) {
			t.Errorf("expected lightbox link, got: %s", got)
		}
	})

	t.Run("skips already wrapped gallery images", func(t *testing.T) {
		html := `<div class="image-gallery"><a data-lightbox="g"><img src="a.jpg"></a></div>`
		got := wrapGalleryContainers(html)
		if got != html {
			t.Errorf("expected no change for already-wrapped gallery, got: %s", got)
		}
	})
}

func TestAddListClasses(t *testing.T) {
	t.Run("adds class to bare ul", func(t *testing.T) {
		got := addListClasses("<ul><li>Item</li></ul>")
		if !strings.Contains(got, `class="list-disc pl-2"`) {
			t.Errorf("expected list-disc class, got: %s", got)
		}
	})

	t.Run("preserves existing class", func(t *testing.T) {
		got := addListClasses(`<ul class="custom"><li>Item</li></ul>`)
		if strings.Contains(got, `list-disc`) {
			t.Errorf("should not add class when one exists, got: %s", got)
		}
	})

	t.Run("adds class to bare ol", func(t *testing.T) {
		got := addListClasses("<ol><li>Item</li></ol>")
		if !strings.Contains(got, `class="list-decimal pl-2"`) {
			t.Errorf("expected list-decimal class, got: %s", got)
		}
	})
}

func TestAddBlockquoteClasses(t *testing.T) {
	t.Run("adds class to bare blockquote", func(t *testing.T) {
		got := addBlockquoteClasses("<blockquote>Quote</blockquote>")
		if !strings.Contains(got, `class="`) {
			t.Errorf("expected class added, got: %s", got)
		}
	})

	t.Run("preserves existing class", func(t *testing.T) {
		input := `<blockquote class="existing">Quote</blockquote>`
		got := addBlockquoteClasses(input)
		if strings.Contains(got, `border-l-4`) {
			t.Errorf("should not add class when one exists, got: %s", got)
		}
	})
}

func TestEmbedYouTube(t *testing.T) {
	t.Run("embeds youtube link", func(t *testing.T) {
		input := `<p><a href="https://www.youtube.com/watch?v=dQw4w9WgXcQ">Video</a></p>`
		got := embedYouTube(input)
		if !strings.Contains(got, "youtube.com/embed/dQw4w9WgXcQ") {
			t.Errorf("expected embedded iframe, got: %s", got)
		}
	})

	t.Run("no change for non-youtube link", func(t *testing.T) {
		input := `<p><a href="https://example.com">Link</a></p>`
		got := embedYouTube(input)
		if got != input {
			t.Errorf("expected no change, got: %s", got)
		}
	})
}

func TestTaskListToHTML(t *testing.T) {
	t.Run("converts unchecked task", func(t *testing.T) {
		input := `<li>[ ] Task one</li>`
		got := taskListToHTML(input)
		if !strings.Contains(got, `type="checkbox"`) {
			t.Errorf("expected checkbox, got: %s", got)
		}
	})

	t.Run("converts checked task", func(t *testing.T) {
		input := `<li>[x] Done task</li>`
		got := taskListToHTML(input)
		if !strings.Contains(got, "checked") {
			t.Errorf("expected checked checkbox, got: %s", got)
		}
	})
}
