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

	"github.com/gorilla/csrf"
)

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
			"add": func(a, b int) int {
				return a + b
			},
			"initial": func(s string) string {
				if s == "" {
					return ""
				}
				r := []rune(s)
				return strings.ToUpper(string(r[0]))
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
	tpl, err := tpl.ParseFS(fs, patterns...)
	if err != nil {
		return Template{}, fmt.Errorf("parsing template: %w", err)
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
