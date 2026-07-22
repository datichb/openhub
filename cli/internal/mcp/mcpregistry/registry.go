// Package mcpregistry provides a registry for MCP servers, supporting both
// built-in native servers and user-installed custom servers.
package mcpregistry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/config"
)

// MCPServer is the interface all MCP server implementations must satisfy.
type MCPServer interface {
	// Name returns the unique server identifier (e.g. "figma", "gitlab").
	Name() string
	// Description returns a short human-readable description.
	Description() string
	// RequiredTokens returns the environment variable names required by this server.
	RequiredTokens() []string
	// Serve starts the MCP server (blocks until the server exits).
	Serve() error
}

// ServerManifest describes a custom MCP server in ~/.oh/mcp/<name>/manifest.json.
type ServerManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Version        string   `json:"version"`
	// Binary is the server binary to execute (relative to manifest dir or absolute).
	Binary         string   `json:"binary"`
	RequiredTokens []string `json:"required_tokens"`
}

// Registry holds all known MCP servers (built-in + custom).
type Registry struct {
	servers map[string]MCPServer
}

// NewRegistry creates a Registry pre-loaded with native servers.
// Call loadBuiltins to register the native servers after creation.
func NewRegistry() *Registry {
	return &Registry{servers: make(map[string]MCPServer)}
}

// Register adds a server to the registry.
func (r *Registry) Register(s MCPServer) {
	r.servers[s.Name()] = s
}

// Get returns a server by name, or nil if not found.
func (r *Registry) Get(name string) MCPServer {
	return r.servers[name]
}

// All returns all registered servers in a stable order (built-in first, then custom).
func (r *Registry) All() []MCPServer {
	builtinOrder := []string{"figma", "gitlab", "gslides", "team"}
	seen := make(map[string]bool)
	var result []MCPServer

	for _, name := range builtinOrder {
		if s, ok := r.servers[name]; ok {
			result = append(result, s)
			seen[name] = true
		}
	}
	for name, s := range r.servers {
		if !seen[name] {
			result = append(result, s)
		}
	}
	return result
}

// Names returns all server names for shell completion.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	return names
}

// CustomMCPDir returns ~/.oh/mcp/.
func CustomMCPDir() string {
	return filepath.Join(config.HubDir(), "mcp")
}

// LoadCustomServers discovers custom MCP servers in ~/.oh/mcp/<name>/manifest.json.
func (r *Registry) LoadCustomServers() {
	dir := CustomMCPDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest ServerManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}
		if manifest.Name == "" || manifest.Binary == "" {
			continue
		}
		serverDir := filepath.Join(dir, entry.Name())
		r.Register(&customMCPServer{
			manifest:  manifest,
			serverDir: serverDir,
		})
	}
}

// customMCPServer implements MCPServer for user-installed external servers.
type customMCPServer struct {
	manifest  ServerManifest
	serverDir string
}

func (c *customMCPServer) Name() string           { return c.manifest.Name }
func (c *customMCPServer) Description() string    { return c.manifest.Description }
func (c *customMCPServer) RequiredTokens() []string { return c.manifest.RequiredTokens }

func (c *customMCPServer) Serve() error {
	binary := c.manifest.Binary
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(c.serverDir, binary)
	}
	if _, err := os.Stat(binary); err != nil {
		return fmt.Errorf("custom MCP server binary not found: %s", binary)
	}
	// Exec-replace the process with the custom server binary
	// so it inherits stdin/stdout for the MCP stdio protocol
	return execBinary(binary)
}
