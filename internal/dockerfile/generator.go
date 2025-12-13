package dockerfile

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/wfaler/devbox/internal/config"
)

// TemplateData holds the data passed to the Dockerfile template
type TemplateData struct {
	LanguageInstalls     string
	BuildSystemInstalls  string
	HasNode              bool
	HasJava              bool
	Env                  map[string]string
	CodeServer           bool
	CodeServerPort       int
	CodeServerTheme      string
	CodeServerExtensions []string
}

// Generate creates a Dockerfile string from the config
func Generate(cfg *config.Config) (string, error) {
	// Build language installation commands
	var langInstalls []string
	var languages []string
	for lang, langCfg := range cfg.Languages {
		languages = append(languages, lang)
		install := GenerateLanguageInstall(lang, langCfg)
		if install != "" {
			langInstalls = append(langInstalls, install)
		}
	}

	// Build build system installation commands
	var bsInstalls []string
	for lang, langCfg := range cfg.Languages {
		install := GenerateBuildSystemInstall(lang, langCfg)
		if install != "" {
			bsInstalls = append(bsInstalls, install)
		}
	}

	// Get VS Code extensions for configured languages if code-server is enabled
	var extensions []string
	if cfg.IsCodeServerEnabled() {
		// Add language-specific extensions
		extensions = GetExtensionsForLanguages(languages)
		// Add custom extensions from config
		extensions = append(extensions, cfg.GetCodeServerExtensions()...)
	}

	data := TemplateData{
		LanguageInstalls:     strings.Join(langInstalls, "\n\n"),
		BuildSystemInstalls:  strings.Join(bsInstalls, "\n\n"),
		HasNode:              cfg.HasLanguage("node"),
		HasJava:              cfg.HasLanguage("java"),
		Env:                  cfg.Env,
		CodeServer:           cfg.IsCodeServerEnabled(),
		CodeServerPort:       cfg.GetCodeServerPort(),
		CodeServerTheme:      cfg.GetCodeServerTheme(),
		CodeServerExtensions: extensions,
	}

	tmpl, err := template.New("dockerfile").Parse(BaseTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}
