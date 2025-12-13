package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the .assistant.yml file
type Config struct {
	Languages      map[string]LanguageConfig `yaml:"languages"`
	Ports          []string                  `yaml:"ports"`
	Env            map[string]string         `yaml:"env"`
	CodeServer     bool                      `yaml:"code_server"`      // Install code-server with language extensions
	CodeServerPort int                       `yaml:"code_server_port"` // Port for code-server (default: 8080)
}

// GetCodeServerPort returns the code-server port, defaulting to 8080
func (c *Config) GetCodeServerPort() int {
	if c.CodeServerPort == 0 {
		return 8080
	}
	return c.CodeServerPort
}

// GetAllPorts returns all configured ports, including code-server port if enabled
func (c *Config) GetAllPorts() []string {
	ports := make([]string, len(c.Ports))
	copy(ports, c.Ports)

	if c.CodeServer {
		csPort := fmt.Sprintf("%d", c.GetCodeServerPort())
		// Check if port is already in the list
		found := false
		for _, p := range ports {
			if p == csPort || strings.HasPrefix(p, csPort+":") || strings.HasSuffix(p, ":"+csPort) {
				found = true
				break
			}
		}
		if !found {
			ports = append(ports, csPort)
		}
	}

	return ports
}

// LanguageConfig defines a language runtime configuration
type LanguageConfig struct {
	Version            string `yaml:"version"`              // "20.10.0", "lts", "latest", or "" (defaults to latest)
	BuildSystem        string `yaml:"build_system"`         // npm, yarn, gradle, etc.
	BuildSystemVersion string `yaml:"build_system_version"` // optional version for build system
}

// SupportedLanguages lists valid language identifiers
var SupportedLanguages = map[string]bool{
	"go":     true,
	"node":   true,
	"rust":   true,
	"java":   true,
	"python": true,
	"ruby":   true,
}

// BuildSystemsForLanguage maps languages to their valid build systems
var BuildSystemsForLanguage = map[string][]string{
	"go":     {}, // built-in
	"node":   {"npm", "yarn", "pnpm"},
	"rust":   {"cargo"},
	"java":   {"gradle", "maven", "ant", "sbt"},
	"python": {"pip", "poetry", "pipenv"},
	"ruby":   {"bundler", "gem"},
}

// Load reads and parses the config file from the given path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return Parse(data)
}

// Parse parses config from YAML bytes
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Initialize maps if nil
	if cfg.Languages == nil {
		cfg.Languages = make(map[string]LanguageConfig)
	}
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
	}

	return &cfg, nil
}

// ExpandEnvVars replaces ${VAR} patterns with host environment values
func (c *Config) ExpandEnvVars() {
	for key, value := range c.Env {
		c.Env[key] = os.Expand(value, os.Getenv)
	}
}

// Validate checks the config for errors
func (c *Config) Validate() error {
	// Validate languages
	for lang, langCfg := range c.Languages {
		if !SupportedLanguages[lang] {
			return fmt.Errorf("unsupported language: %s (supported: go, node, rust, java, python, ruby)", lang)
		}

		if langCfg.BuildSystem != "" {
			validSystems := BuildSystemsForLanguage[lang]
			if !contains(validSystems, langCfg.BuildSystem) {
				return fmt.Errorf("invalid build system %q for language %s (valid: %s)",
					langCfg.BuildSystem, lang, strings.Join(validSystems, ", "))
			}
		}
	}

	// Validate port format
	for _, port := range c.Ports {
		if err := validatePortSpec(port); err != nil {
			return fmt.Errorf("invalid port %q: %w", port, err)
		}
	}

	return nil
}

// HasLanguage checks if a specific language is configured
func (c *Config) HasLanguage(lang string) bool {
	_, ok := c.Languages[lang]
	return ok
}

// GetVersion returns the version for a language, defaulting to "latest" if not specified
func (lc *LanguageConfig) GetVersion() string {
	if lc.Version == "" {
		return "latest"
	}
	return lc.Version
}

// validatePortSpec validates a port specification in format "port" or "host:container"
func validatePortSpec(spec string) error {
	parts := strings.Split(spec, ":")

	switch len(parts) {
	case 1:
		// Single port: "8080"
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return fmt.Errorf("invalid port number: %s", parts[0])
		}
	case 2:
		// Host:Container mapping: "8080:8080"
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return fmt.Errorf("invalid host port: %s", parts[0])
		}
		if _, err := strconv.Atoi(parts[1]); err != nil {
			return fmt.Errorf("invalid container port: %s", parts[1])
		}
	default:
		return fmt.Errorf("invalid format, expected 'port' or 'host:container'")
	}

	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
