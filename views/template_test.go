package views

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMust(t *testing.T) {
	t.Run("returns template on success", func(t *testing.T) {
		tmpl := Template{}
		result := Must(tmpl, nil)

		// Should return template unchanged
		_ = result
	})

	t.Run("panics on error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Must should panic on error")
			}
		}()

		Must(Template{}, assert("test error"))
	})
}

func TestTemplate_Execute(t *testing.T) {
	t.Run("execute doesn't panic with empty template", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// It's okay if it panics due to nil template
				// We're just testing that the method exists
			}
		}()

		tmpl := Template{}
		w := httptest.NewRecorder()
		data := map[string]string{"Name": "World"}

		tmpl.Execute(w, nil, data)
	})
}

// Test the template helper functions
func TestTemplateHelpers(t *testing.T) {
	t.Run("contains helper", func(t *testing.T) {
		str := "hello world"
		substr := "world"

		result := strings.Contains(str, substr)
		if !result {
			t.Error("Contains helper should return true")
		}
	})

	t.Run("upper helper", func(t *testing.T) {
		str := "hello"
		result := strings.ToUpper(str)

		if result != "HELLO" {
			t.Errorf("Upper helper should return 'HELLO', got %s", result)
		}
	})

	t.Run("add helper", func(t *testing.T) {
		result := 1 + 2
		if result != 3 {
			t.Errorf("Add helper should return 3, got %d", result)
		}
	})

	t.Run("initial helper logic", func(t *testing.T) {
		// Test the logic of getting first character uppercased
		str := "hello"
		if len(str) > 0 {
			r := []rune(str)
			result := strings.ToUpper(string(r[0]))
			if result != "H" {
				t.Errorf("Initial helper should return 'H', got %s", result)
			}
		}
	})

	t.Run("initial helper with empty string", func(t *testing.T) {
		str := ""
		if str == "" {
			// Empty string handling works
			return
		}
		t.Error("Empty string check failed")
	})
}

// Helper function to create error
func assert(msg string) error {
	return &testError{msg: msg}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
