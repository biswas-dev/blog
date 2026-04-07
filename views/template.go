package views

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"

	"anshumanbiswas.com/blog/icons"
	"github.com/gorilla/csrf"
)

// SiteConfigFunc is a package-level hook that templates call via the
// "siteConfig" FuncMap entry. Set it during init (main.go) to point at
// SiteSettingsService.Get. When nil the function returns the fallback.
var SiteConfigFunc func(key, fallback string) string

type Template struct {
	htmlTpl *template.Template
}

func Must(t Template, err error) Template {
	if err != nil {
		panic(err)
	}
	return t
}

func ParseFS(fs fs.FS, patterns ...string) (Template, error) {
	tpl := template.New(filepath.Base(patterns[0]))
	tpl = tpl.Funcs(
		template.FuncMap{
			"csrfField": func() template.HTML {
				return `<input type="hidden" />`
			},
			"contains": func(s, substr string) bool {
				return strings.Contains(s, substr)
			},
			"upper": func(s string) string { return strings.ToUpper(s) },
			"icon": func(name, class string) template.HTML {
				return icons.Icon(name, class)
			},
			"ratingStars": func(rating float64) template.HTML {
				var b strings.Builder
				for i := 1; i <= 5; i++ {
					fi := float64(i)
					if rating >= fi {
						b.WriteString(`<span style="color:#f59e0b;">&#9733;</span>`)
					} else if rating >= fi-0.5 {
						// Half star: full star clipped to left half + empty star clipped to right half
						b.WriteString(`<span style="position:relative;display:inline-block;color:#d1d5db;">&#9733;<span style="position:absolute;left:0;top:0;overflow:hidden;width:50%;color:#f59e0b;">&#9733;</span></span>`)
					} else {
						b.WriteString(`<span style="color:#d1d5db;">&#9733;</span>`)
					}
				}
				return template.HTML(b.String())
			},
			"add": func(a, b int) int {
				return a + b
			},
			"thumbURL": func(url string) string {
				if url == "" {
					return ""
				}
				ext := filepath.Ext(url)
				return url[:len(url)-len(ext)] + "_thumb" + ext
			},
			"initial": func(s string) string {
				if s == "" {
					return ""
				}
				r := []rune(s)
				return strings.ToUpper(string(r[0]))
			},
			"siteConfig": func(key, fallback string) string {
				if SiteConfigFunc != nil {
					return SiteConfigFunc(key, fallback)
				}
				return fallback
			},
			"where": func(slice interface{}, field string, value interface{}) interface{} {
				sliceValue := reflect.ValueOf(slice)
				if sliceValue.Kind() != reflect.Slice {
					return slice
				}

				result := reflect.MakeSlice(sliceValue.Type(), 0, 0)
				for i := 0; i < sliceValue.Len(); i++ {
					item := sliceValue.Index(i)
					if item.Kind() == reflect.Ptr {
						item = item.Elem()
					}

					if item.Kind() == reflect.Struct {
						fieldValue := item.FieldByName(field)
						if fieldValue.IsValid() && reflect.DeepEqual(fieldValue.Interface(), value) {
							result = reflect.Append(result, sliceValue.Index(i))
						}
					}
				}
				return result.Interface()
			},
		},
	)
	// Parse layout templates (index 1+) before the page template (index 0).
	// This allows page-level {{define}} to override layout {{block}} defaults
	// (e.g., og-meta, page-title) since the last-parsed definition wins.
	for i := len(patterns) - 1; i >= 0; i-- {
		var err error
		tpl, err = tpl.ParseFS(fs, patterns[i])
		if err != nil {
			return Template{}, fmt.Errorf("parsing template %s: %w", patterns[i], err)
		}
	}
	return Template{
		htmlTpl: tpl,
	}, nil
}

func (t Template) Execute(w http.ResponseWriter, r *http.Request, data interface{}) {
	tpl, err := t.htmlTpl.Clone()
	if err != nil {
		log.Printf("cloning template: %v", err)
		http.Error(w, "There was an error rendering the page.", http.StatusInternalServerError)
		return
	}
	// Only re-register csrfField since it needs the per-request *http.Request.
	// All other funcs (contains, upper, add, initial, where) are already registered at parse time.
	tpl = tpl.Funcs(
		template.FuncMap{
			"csrfField": func() template.HTML {
				return csrf.TemplateField(r)
			},
		},
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	err = tpl.Execute(&buf, data)
	if err != nil {
		log.Printf("executing template: %v", err)
		http.Error(w, "There was an error executing the template.", http.StatusInternalServerError)
		return
	}
	io.Copy(w, &buf)
}
