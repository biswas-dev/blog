package themes

import (
    "fmt"
    "html/template"
)

// Theme represents a blog theme configuration
type Theme struct {
    Name        string
    DisplayName string
    Description string
    Author      string
    Version     string
    Templates   map[string]string
    Assets      map[string]string
}

// ThemeManager manages different blog themes
type ThemeManager struct {
    themes        map[string]*Theme
    currentTheme  string
    templateCache map[string]*template.Template
}

// NewThemeManager creates a new theme manager
func NewThemeManager() *ThemeManager {
    tm := &ThemeManager{
        themes:        make(map[string]*Theme),
        templateCache: make(map[string]*template.Template),
        currentTheme:  "modern", // default theme
    }
    
    // Register built-in themes
    tm.registerBuiltinThemes()
    
    return tm
}

// registerBuiltinThemes registers the built-in themes
func (tm *ThemeManager) registerBuiltinThemes() {
    // Modern theme
    modern := &Theme{
        Name:        "modern",
        DisplayName: "Modern",
        Description: "A clean, modern theme inspired by anshumanbiswas.com with dark mode support",
        Author:      "Anshuman Biswas",
        Version:     "1.0.0",
        Templates: map[string]string{
            "base":     "themes/modern/base.gohtml",
            "home":     "themes/modern/home.gohtml",
            "post":     "themes/modern/blogpost.gohtml",
            "admin":    "themes/modern/admin.gohtml",
            "signin":   "themes/modern/signin.gohtml",
            "signup":   "themes/modern/signup.gohtml",
            "about":    "themes/modern/about.gohtml",
            "404":      "themes/modern/404.gohtml",
        },
        Assets: map[string]string{
            "css":    "/static/css/theme.css",
            "js":     "/static/js/modern-theme.js",
            "favicon": "/static/favicon.svg",
            "logo":   "/static/logo.svg",
        },
    }
    tm.RegisterTheme(modern)
    
    // Classic theme (fallback to original templates)
    classic := &Theme{
        Name:        "classic",
        DisplayName: "Classic",
        Description: "The original blog theme with basic styling",
        Author:      "Anshuman Biswas",
        Version:     "1.0.0",
        Templates: map[string]string{
            "base":     "templates/tailwind.gohtml",
            "home":     "templates/home.gohtml",
            "post":     "templates/blogpost.gohtml",
            "admin":    "templates/admin.gohtml",
            "signin":   "templates/signin.gohtml",
            "signup":   "templates/signup.gohtml",
            "about":    "templates/about.gohtml",
            "404":      "templates/NotFoundPage.gohtml",
        },
        Assets: map[string]string{
            "css": "/css/main.css",
        },
    }
    tm.RegisterTheme(classic)
}

// RegisterTheme registers a new theme
func (tm *ThemeManager) RegisterTheme(theme *Theme) {
    tm.themes[theme.Name] = theme
}

// GetTheme returns a theme by name
func (tm *ThemeManager) GetTheme(name string) (*Theme, error) {
    theme, exists := tm.themes[name]
    if !exists {
        return nil, fmt.Errorf("theme '%s' not found", name)
    }
    return theme, nil
}

// SetCurrentTheme sets the active theme
func (tm *ThemeManager) SetCurrentTheme(themeName string) error {
    _, exists := tm.themes[themeName]
    if !exists {
        return fmt.Errorf("theme '%s' not found", themeName)
    }
    tm.currentTheme = themeName
    return nil
}

// GetCurrentTheme returns the current active theme
func (tm *ThemeManager) GetCurrentTheme() *Theme {
    return tm.themes[tm.currentTheme]
}

// GetAvailableThemes returns all available themes
func (tm *ThemeManager) GetAvailableThemes() map[string]*Theme {
    return tm.themes
}

// GetTemplate returns the template path for a specific template type
func (tm *ThemeManager) GetTemplate(templateType string) (string, error) {
    theme := tm.GetCurrentTheme()
    if theme == nil {
        return "", fmt.Errorf("no active theme")
    }
    
    templatePath, exists := theme.Templates[templateType]
    if !exists {
        return "", fmt.Errorf("template '%s' not found in theme '%s'", templateType, theme.Name)
    }
    
    return templatePath, nil
}

// GetAsset returns the asset path for a specific asset type
func (tm *ThemeManager) GetAsset(assetType string) string {
    theme := tm.GetCurrentTheme()
    if theme == nil {
        return ""
    }
    
    assetPath, exists := theme.Assets[assetType]
    if !exists {
        return ""
    }
    
    return assetPath
}

// LoadTemplates loads and caches templates for the current theme
func (tm *ThemeManager) LoadTemplates() error {
    theme := tm.GetCurrentTheme()
    if theme == nil {
        return fmt.Errorf("no active theme")
    }
    
    // Clear existing cache
    tm.templateCache = make(map[string]*template.Template)
    
    // Load templates
    for templateType, templatePath := range theme.Templates {
        tmpl, err := template.ParseFiles(templatePath)
        if err != nil {
            // Try fallback to classic theme template
            if theme.Name != "classic" {
                classicTheme := tm.themes["classic"]
                if classicPath, exists := classicTheme.Templates[templateType]; exists {
                    tmpl, err = template.ParseFiles(classicPath)
                }
            }
            if err != nil {
                return fmt.Errorf("failed to load template '%s': %v", templateType, err)
            }
        }
        tm.templateCache[templateType] = tmpl
    }
    
    return nil
}

// GetCachedTemplate returns a cached template
func (tm *ThemeManager) GetCachedTemplate(templateType string) (*template.Template, error) {
    tmpl, exists := tm.templateCache[templateType]
    if !exists {
        return nil, fmt.Errorf("template '%s' not cached", templateType)
    }
    return tmpl, nil
}

// GetThemeContext returns context data for templates
func (tm *ThemeManager) GetThemeContext() map[string]interface{} {
    theme := tm.GetCurrentTheme()
    return map[string]interface{}{
        "Theme":     theme,
        "ThemeName": theme.Name,
        "Assets":    theme.Assets,
    }
}
