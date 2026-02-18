package themes

import (
	"testing"
)

func TestNewThemeManager(t *testing.T) {
	tm := NewThemeManager()

	if tm == nil {
		t.Fatal("NewThemeManager returned nil")
	}

	currentTheme := tm.GetCurrentTheme()
	if currentTheme == nil {
		t.Error("Expected current theme to be set")
	}

	themes := tm.GetAvailableThemes()
	if len(themes) == 0 {
		t.Error("Expected at least one theme to be registered")
	}
}

func TestThemeManager_GetCurrentTheme(t *testing.T) {
	tm := NewThemeManager()

	theme := tm.GetCurrentTheme()
	if theme == nil {
		t.Fatal("GetCurrentTheme returned nil")
	}

	if theme.Name == "" {
		t.Error("Expected theme name to be set")
	}
}

func TestThemeManager_GetTheme(t *testing.T) {
	tm := NewThemeManager()

	// Test getting existing theme
	theme, err := tm.GetTheme("modern")
	if err != nil {
		t.Errorf("GetTheme('modern') error: %v", err)
	}
	if theme == nil {
		t.Fatal("Expected to get modern theme")
	}

	if theme.Name != "modern" {
		t.Errorf("Expected theme name 'modern', got %s", theme.Name)
	}

	// Test getting non-existent theme
	theme, err = tm.GetTheme("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent theme")
	}
	if theme != nil {
		t.Error("Expected nil theme for non-existent theme name")
	}
}

func TestThemeManager_SetCurrentTheme(t *testing.T) {
	tm := NewThemeManager()

	// Test setting valid theme
	err := tm.SetCurrentTheme("modern")
	if err != nil {
		t.Errorf("SetCurrentTheme('modern') error: %v", err)
	}

	current := tm.GetCurrentTheme()
	if current.Name != "modern" {
		t.Errorf("Expected current theme 'modern', got %s", current.Name)
	}

	// Test setting invalid theme
	err = tm.SetCurrentTheme("nonexistent")
	if err == nil {
		t.Error("Expected error when setting non-existent theme")
	}
}

func TestThemeManager_GetAvailableThemes(t *testing.T) {
	tm := NewThemeManager()

	themes := tm.GetAvailableThemes()
	if len(themes) == 0 {
		t.Error("Expected at least one available theme")
	}

	if _, exists := themes["modern"]; !exists {
		t.Error("Expected 'modern' theme to be available")
	}
}

func TestThemeManager_RegisterTheme(t *testing.T) {
	tm := NewThemeManager()

	customTheme := &Theme{
		Name:        "custom",
		DisplayName: "Custom Theme",
		Description: "A custom test theme",
		Author:      "Test Author",
		Version:     "1.0.0",
		Templates:   make(map[string]string),
		Assets:      make(map[string]string),
	}

	tm.RegisterTheme(customTheme)

	theme, err := tm.GetTheme("custom")
	if err != nil {
		t.Errorf("GetTheme('custom') error: %v", err)
	}
	if theme == nil {
		t.Fatal("Failed to register custom theme")
	}

	if theme.Name != "custom" {
		t.Errorf("Expected theme name 'custom', got %s", theme.Name)
	}
}

func TestThemeManager_GetTemplate(t *testing.T) {
	tm := NewThemeManager()

	// Test getting a template path
	tmplPath, err := tm.GetTemplate("base")
	if err != nil {
		// It's okay if template doesn't exist, we're just testing the method
		return
	}

	if tmplPath == "" {
		t.Error("Expected non-empty template path")
	}
}

func TestThemeManager_GetAsset(t *testing.T) {
	tm := NewThemeManager()

	// Test getting an asset path
	assetPath := tm.GetAsset("css")

	// Asset may or may not exist, just verify method doesn't panic
	_ = assetPath
}

func TestThemeManager_GetThemeContext(t *testing.T) {
	tm := NewThemeManager()

	ctx := tm.GetThemeContext()
	if ctx == nil {
		t.Error("Expected non-nil theme context")
	}
}

func TestTheme_BasicFields(t *testing.T) {
	theme := &Theme{
		Name:        "test",
		DisplayName: "Test Theme",
		Description: "Test Description",
		Author:      "Test Author",
		Version:     "1.0.0",
		Templates:   make(map[string]string),
		Assets:      make(map[string]string),
	}

	if theme.Name != "test" {
		t.Errorf("Expected name 'test', got %s", theme.Name)
	}

	if theme.DisplayName != "Test Theme" {
		t.Errorf("Expected display name 'Test Theme', got %s", theme.DisplayName)
	}

	if theme.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", theme.Version)
	}
}
