package dockerfile

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/wfaler/rig/internal/config"
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
	MarkdownServer       bool
	MarkdownServerPort   int
	Shell                string
}

// BuildContext holds the Dockerfile and any extra files needed in the build context
type BuildContext struct {
	Dockerfile string
	ExtraFiles map[string][]byte // filename -> content
}

// Generate creates a Dockerfile string from the config
func Generate(cfg *config.Config) (*BuildContext, error) {
	// Build language installation commands
	var langInstalls []string
	for lang, langCfg := range cfg.Languages {
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

	// Get VS Code extensions from config (user specifies all extensions explicitly)
	var extensions []string
	if cfg.IsCodeServerEnabled() {
		extensions = cfg.GetCodeServerExtensions()
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
		MarkdownServer:       cfg.IsMarkdownServerEnabled(),
		MarkdownServerPort:   cfg.GetMarkdownServerPort(),
		Shell:                cfg.GetShell(),
	}

	tmpl, err := template.New("dockerfile").Parse(BaseTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	ctx := &BuildContext{
		Dockerfile: buf.String(),
		ExtraFiles: make(map[string][]byte),
	}

	if cfg.IsMarkdownServerEnabled() {
		ctx.ExtraFiles["rig-md-server.js"] = []byte(GetMarkdownServerScript())
	}

	return ctx, nil
}
