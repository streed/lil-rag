package theme

import (
	"embed"
	"html/template"
	"io"
)

//go:embed templates/*
var templateFS embed.FS

// Theme represents the application theme configuration
type Theme struct {
	Name               string
	PrimaryColor       string
	SecondaryColor     string
	AccentColor        string
	BackgroundGradient string
	FontFamily         string
}

// DefaultTheme returns the default theme configuration
func DefaultTheme() *Theme {
	return &Theme{
		Name:               "Default",
		PrimaryColor:       "#007AFF",
		SecondaryColor:     "#667eea",
		AccentColor:        "#764ba2",
		BackgroundGradient: "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
		FontFamily:         "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif",
	}
}

// TemplateData represents data passed to templates
type TemplateData struct {
	Title       string
	Version     string
	Theme       *Theme
	PageContent interface{}
	PageName    string
	Data        interface{} // Additional data for templates
}

// Renderer handles template rendering with theme support
type Renderer struct {
	templates *template.Template
	theme     *Theme
}

// NewRenderer creates a new template renderer
func NewRenderer() (*Renderer, error) {
	templates, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Renderer{
		templates: templates,
		theme:     DefaultTheme(),
	}, nil
}

// SetTheme updates the current theme
func (r *Renderer) SetTheme(theme *Theme) {
	r.theme = theme
}

// RenderPage renders a page template with theme data
func (r *Renderer) RenderPage(w io.Writer, templateName string, data *TemplateData) error {
	if data.Theme == nil {
		data.Theme = r.theme
	}

	// Don't override PageContent if it's already set - it contains the page-specific template to render
	// The templateName parameter is the base template to execute

	// Execute the base template, which will include the page-specific content blocks
	return r.templates.ExecuteTemplate(w, templateName, data)
}

// GetTheme returns the current theme
func (r *Renderer) GetTheme() *Theme {
	return r.theme
}
