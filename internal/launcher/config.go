package launcher

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// Tool is a startable entry from tools.json.
type Tool struct {
	Name         string            `json:"name"`
	Group        string            `json:"group"`
	Dir          string            `json:"dir"`
	Start        string            `json:"start"`
	StartWindows string            `json:"start_windows,omitempty"`
	URL          string            `json:"url"`
	Env          map[string]string `json:"env,omitempty"`
}

// Link is a plain URL entry from tools.json.
type Link struct {
	Name  string `json:"name"`
	Group string `json:"group"`
	URL   string `json:"url"`
}

// Config is the parsed tools.json.
type Config struct {
	Tools []Tool `json:"tools"`
	Links []Link `json:"links"`

	baseDir string // directory containing tools.json
}

// LoadConfig reads and validates tools.json at path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	cfg.baseDir = filepath.Dir(abs)

	seen := map[string]bool{}
	for i, t := range cfg.Tools {
		if t.Name == "" {
			return nil, fmt.Errorf("tool #%d: missing name", i+1)
		}
		if seen[t.Name] {
			return nil, fmt.Errorf("tool %q: duplicate name", t.Name)
		}
		seen[t.Name] = true
		if t.Start == "" {
			return nil, fmt.Errorf("tool %q: missing start", t.Name)
		}
		if _, err := ToolPort(t.URL); err != nil {
			return nil, fmt.Errorf("tool %q: %w", t.Name, err)
		}
	}
	for i, l := range cfg.Links {
		if l.Name == "" || l.URL == "" {
			return nil, fmt.Errorf("link #%d: name and url are required", i+1)
		}
	}
	return &cfg, nil
}

// ToolDir returns the absolute working directory of a tool.
func (c *Config) ToolDir(t Tool) string {
	if filepath.IsAbs(t.Dir) {
		return filepath.Clean(t.Dir)
	}
	return filepath.Join(c.baseDir, t.Dir)
}

// Tool looks up a tool by name.
func (c *Config) Tool(name string) (Tool, bool) {
	for _, t := range c.Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// toolPort extracts the port from a tool URL, defaulting by scheme.
func ToolPort(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid url %q", raw)
	}
	if p := u.Port(); p != "" {
		return p, nil
	}
	switch u.Scheme {
	case "http":
		return "80", nil
	case "https":
		return "443", nil
	}
	return "", fmt.Errorf("url %q has no port", raw)
}

// toolAddr returns host:port to probe for a tool URL.
func toolAddr(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	port, err := ToolPort(raw)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	return host + ":" + port, nil
}
