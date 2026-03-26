package dockerfile

import (
	_ "embed"
	"strings"
)

//go:embed scripts/rig-md-server.js
var markdownServerScript string

//go:embed scripts/rig-md-client.js
var markdownClientJS string

//go:embed scripts/rig-md-styles.css
var markdownCSS string

// MarkdownServerScript returns the server JS with CSS and client JS injected.
func GetMarkdownServerScript() string {
	// Escape the CSS and client JS for embedding as JS string literals
	cssEscaped := escapeForJSString(markdownCSS)
	clientJSEscaped := escapeForJSString(markdownClientJS)

	s := markdownServerScript
	s = strings.Replace(s, "RIG_INJECTED_CSS", "`"+cssEscaped+"`", 1)
	s = strings.Replace(s, "RIG_INJECTED_CLIENT_JS", "`"+clientJSEscaped+"`", 1)
	return s
}

// escapeForJSString escapes content for safe embedding inside a JS template literal (backticks).
func escapeForJSString(s string) string {
	// Escape backticks and ${} template expressions
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "${", "\\${")
	return s
}
