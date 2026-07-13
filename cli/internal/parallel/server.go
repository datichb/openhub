package parallel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// OpenCodeServer manages a single opencode serve instance.
type OpenCodeServer struct {
	Port        int
	Dir         string // worktree directory
	BaseURL     string
	Process     *exec.Cmd
	TicketID    string
	cancelFunc  context.CancelFunc
	httpClient  *http.Client
}

// NewServer creates a new server config (not yet started).
func NewServer(port int, dir, ticketID string) *OpenCodeServer {
	return &OpenCodeServer{
		Port:     port,
		Dir:      dir,
		BaseURL:  fmt.Sprintf("http://127.0.0.1:%d", port),
		TicketID: ticketID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start launches the opencode serve subprocess.
func (s *OpenCodeServer) Start(ctx context.Context, opencodeBin string) error {
	srvCtx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	args := []string{
		"serve",
		"--port", fmt.Sprintf("%d", s.Port),
		"--hostname", "127.0.0.1",
	}

	s.Process = exec.CommandContext(srvCtx, opencodeBin, args...)
	s.Process.Dir = s.Dir
	s.Process.Env = append(os.Environ(), "OPENCODE_DISABLE_AUTOUPDATE=true")
	// Discard output (server logs go to its own log file)
	s.Process.Stdout = io.Discard
	s.Process.Stderr = io.Discard

	if err := s.Process.Start(); err != nil {
		cancel()
		return fmt.Errorf("starting opencode serve on port %d: %w", s.Port, err)
	}

	return nil
}

// WaitReady polls the health endpoint until the server is ready.
func (s *OpenCodeServer) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := s.httpClient.Get(s.BaseURL + "/global/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("opencode serve on port %d did not become ready within %s", s.Port, timeout)
}

// CreateSession creates a new session on the server.
func (s *OpenCodeServer) CreateSession(title string) (string, error) {
	body := fmt.Sprintf(`{"title":%q}`, title)
	resp, err := s.post("/session", body)
	if err != nil {
		return "", fmt.Errorf("creating session: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding session response: %w", err)
	}
	return result.ID, nil
}

// SendPromptAsync sends a prompt asynchronously (fire and forget).
func (s *OpenCodeServer) SendPromptAsync(sessionID, prompt, agent string) error {
	bodyMap := map[string]interface{}{
		"parts": []map[string]string{
			{"type": "text", "text": prompt},
		},
	}
	if agent != "" {
		bodyMap["agent"] = agent
	}

	data, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	resp, err := s.post(fmt.Sprintf("/session/%s/prompt_async", sessionID), string(data))
	if err != nil {
		return fmt.Errorf("sending prompt: %w", err)
	}
	resp.Body.Close()
	return nil
}

// GetSessionStatus returns the status of all sessions.
func (s *OpenCodeServer) GetSessionStatus() (map[string]string, error) {
	resp, err := s.get("/session/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	statuses := make(map[string]string)
	for id, v := range result {
		if status, ok := v.(string); ok {
			statuses[id] = status
		} else if m, ok := v.(map[string]interface{}); ok {
			if s, ok := m["status"].(string); ok {
				statuses[id] = s
			}
		}
	}
	return statuses, nil
}

// GetFileStatus returns the list of modified files (git status).
func (s *OpenCodeServer) GetFileStatus() ([]string, error) {
	resp, err := s.get("/file/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var files []struct {
		Path   string `json:"path"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		// Try array of strings fallback
		var paths []string
		return paths, nil
	}

	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	return paths, nil
}

// AbortSession aborts a running session.
func (s *OpenCodeServer) AbortSession(sessionID string) error {
	resp, err := s.post(fmt.Sprintf("/session/%s/abort", sessionID), "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Dispose shuts down the server cleanly.
func (s *OpenCodeServer) Dispose() error {
	resp, err := s.post("/instance/dispose", "")
	if err != nil {
		// Server might already be down
		return nil
	}
	resp.Body.Close()
	return nil
}

// Kill force-kills the server process.
func (s *OpenCodeServer) Kill() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	if s.Process != nil && s.Process.Process != nil {
		_ = s.Process.Process.Kill()
	}
}

// IsAlive checks if the server process is still running.
func (s *OpenCodeServer) IsAlive() bool {
	if s.Process == nil || s.Process.Process == nil {
		return false
	}
	resp, err := s.httpClient.Get(s.BaseURL + "/global/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// --- HTTP helpers ---

func (s *OpenCodeServer) get(path string) (*http.Response, error) {
	resp, err := s.httpClient.Get(s.BaseURL + path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func (s *OpenCodeServer) post(path, body string) (*http.Response, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req, err := http.NewRequest("POST", s.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return resp, nil
}
