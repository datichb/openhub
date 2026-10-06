// Package gitlabtest is an in-memory fake of the GitLab REST API subset used
// by the remote runtime (httptest), shared by the tests of several packages.
package gitlabtest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Variable is a stored CI variable.
type Variable struct {
	Key       string
	Value     string
	Masked    bool
	Protected bool
	Raw       bool
}

// Project is a fake project.
type Project struct {
	ID            int64
	Path          string // full path
	DefaultBranch string
	Packages      bool
	Registry      string // disabled | private | enabled
	Files         map[string][]byte
	Commits       int
	Variables     map[string]Variable
	Triggers      []map[string]any
	Runners       []map[string]any  // id, description, status, online, tags ([]string)
	Generic       map[string][]byte // name/version/file → content
	Unprotected   bool              // default branch not protected
}

// Server is the fake GitLab.
type Server struct {
	*httptest.Server
	mu       sync.Mutex
	Token    string // accepted PRIVATE-TOKEN ("" = any)
	User     string
	Groups   map[string]int64 // full path → id
	Projects map[string]*Project
	nextID   int64
	// Requests records "METHOD path" of every call.
	Requests []string
	// Fail forces a status for "METHOD path-prefix".
	Fail map[string]int
}

// New starts a fake GitLab with one group.
func New(t testing.TB, group string) *Server {
	s := &Server{
		User:     "alice",
		Groups:   map[string]int64{group: 10},
		Projects: map[string]*Project{},
		nextID:   100,
		Fail:     map[string]int{},
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// AddProject registers a project.
func (s *Server) AddProject(path string) *Project {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addProject(path)
}

func (s *Server) addProject(path string) *Project {
	s.nextID++
	p := &Project{ID: s.nextID, Path: path, DefaultBranch: "main", Packages: true, Registry: "private",
		Files: map[string][]byte{}, Variables: map[string]Variable{}, Generic: map[string][]byte{}}
	s.Projects[path] = p
	return p
}

// Project returns a project by path (nil if absent).
func (s *Server) Project(path string) *Project {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Projects[path]
}

func (s *Server) findProject(ref string) *Project {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		for _, p := range s.Projects {
			if p.ID == id {
				return p
			}
		}
		return nil
	}
	return s.Projects[ref]
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"message": "404 Not Found"})
}

// segments splits the escaped path after /api/v4/ and unescapes each part.
func segments(r *http.Request) []string {
	raw := strings.TrimPrefix(r.URL.EscapedPath(), "/api/v4/")
	parts := strings.Split(raw, "/")
	for i, p := range parts {
		if u, err := url.PathUnescape(p); err == nil {
			parts[i] = u
		}
	}
	return parts
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := strings.TrimPrefix(r.URL.EscapedPath(), "/api/v4")
	s.Requests = append(s.Requests, r.Method+" "+path)
	for k, code := range s.Fail {
		if strings.HasPrefix(r.Method+" "+path, k) {
			writeJSON(w, code, map[string]string{"message": "forced failure"})
			return
		}
	}
	if s.Token != "" && r.Header.Get("PRIVATE-TOKEN") != s.Token && r.Header.Get("JOB-TOKEN") != s.Token {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "401 Unauthorized"})
		return
	}
	seg := segments(r)
	switch {
	case len(seg) == 1 && seg[0] == "user":
		writeJSON(w, 200, map[string]any{"id": 1, "username": s.User, "name": "Alice"})
	case len(seg) == 2 && seg[0] == "groups":
		id, ok := s.Groups[seg[1]]
		if !ok {
			notFound(w)
			return
		}
		writeJSON(w, 200, map[string]any{"id": id, "full_path": seg[1]})
	case len(seg) == 1 && seg[0] == "projects" && r.Method == http.MethodPost:
		s.createProject(w, r)
	case len(seg) >= 2 && seg[0] == "projects":
		p := s.findProject(seg[1])
		if p == nil {
			notFound(w)
			return
		}
		s.serveProject(w, r, p, seg[2:])
	default:
		notFound(w)
	}
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	ns, _ := body["namespace_id"].(float64)
	group := ""
	for g, id := range s.Groups {
		if float64(id) == ns {
			group = g
		}
	}
	if group == "" {
		writeJSON(w, 400, map[string]string{"message": "namespace not found"})
		return
	}
	name, _ := body["path"].(string)
	p := s.addProject(group + "/" + name)
	p.Files["README.md"] = []byte("# " + name)
	writeJSON(w, 201, s.projectJSON(p))
}

func (s *Server) projectJSON(p *Project) map[string]any {
	return map[string]any{
		"id": p.ID, "path_with_namespace": p.Path, "default_branch": p.DefaultBranch,
		"web_url": s.URL + "/" + p.Path, "http_url_to_repo": s.URL + "/" + p.Path + ".git",
		"packages_enabled": p.Packages, "container_registry_access_level": p.Registry,
		"container_registry_image_prefix": "registry.example.com/" + p.Path,
		"empty_repo":                      len(p.Files) == 0,
	}
}

func (s *Server) serveProject(w http.ResponseWriter, r *http.Request, p *Project, rest []string) {
	switch {
	case len(rest) == 0:
		writeJSON(w, 200, s.projectJSON(p))
	case len(rest) == 3 && rest[0] == "repository" && rest[1] == "files":
		s.serveFile(w, r, p, rest[2])
	case len(rest) >= 1 && rest[0] == "variables":
		s.serveVariables(w, r, p, rest[1:])
	case len(rest) == 1 && rest[0] == "triggers":
		if r.Method == http.MethodPost {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			t := map[string]any{"id": len(p.Triggers) + 1, "description": body["description"], "token": fmt.Sprintf("glptt-%d-secret", len(p.Triggers)+1)}
			p.Triggers = append(p.Triggers, t)
			writeJSON(w, 201, t)
			return
		}
		writeJSON(w, 200, p.Triggers)
	case len(rest) == 1 && rest[0] == "runners":
		tag := r.URL.Query().Get("tag_list")
		out := []map[string]any{}
		for _, rn := range p.Runners {
			if tag != "" {
				tags, _ := rn["tags"].([]string)
				found := false
				for _, t := range tags {
					found = found || t == tag
				}
				if !found {
					continue
				}
			}
			if r.URL.Query().Get("status") == "online" && rn["online"] != true {
				continue
			}
			out = append(out, rn)
		}
		writeJSON(w, 200, out)
	case len(rest) == 2 && rest[0] == "protected_branches":
		if p.Unprotected || rest[1] != p.DefaultBranch {
			notFound(w)
			return
		}
		writeJSON(w, 200, map[string]any{"name": rest[1]})
	case len(rest) == 5 && rest[0] == "packages" && rest[1] == "generic":
		s.serveGeneric(w, r, p, rest[2]+"/"+rest[3]+"/"+rest[4])
	default:
		notFound(w)
	}
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, p *Project, name string) {
	switch r.Method {
	case http.MethodGet:
		data, ok := p.Files[name]
		if !ok {
			notFound(w)
			return
		}
		writeJSON(w, 200, map[string]string{"content": base64.StdEncoding.EncodeToString(data), "encoding": "base64"})
	case http.MethodPost, http.MethodPut:
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, exists := p.Files[name]
		if r.Method == http.MethodPost && exists {
			writeJSON(w, 400, map[string]string{"message": "A file with this name already exists"})
			return
		}
		if r.Method == http.MethodPut && !exists {
			writeJSON(w, 400, map[string]string{"message": "A file with this name doesn't exist"})
			return
		}
		data, err := base64.StdEncoding.DecodeString(body["content"])
		if err != nil || body["encoding"] != "base64" || body["branch"] == "" {
			writeJSON(w, 400, map[string]string{"message": "bad request"})
			return
		}
		p.Files[name] = data
		p.Commits++
		writeJSON(w, 201, map[string]string{"file_path": name})
	default:
		notFound(w)
	}
}

func (s *Server) serveVariables(w http.ResponseWriter, r *http.Request, p *Project, rest []string) {
	asJSON := func(v Variable) map[string]any {
		return map[string]any{"key": v.Key, "value": v.Value, "masked": v.Masked, "protected": v.Protected, "raw": v.Raw}
	}
	decode := func() Variable {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		v := Variable{}
		v.Key, _ = body["key"].(string)
		v.Value, _ = body["value"].(string)
		v.Masked, _ = body["masked"].(bool)
		v.Protected, _ = body["protected"].(bool)
		v.Raw, _ = body["raw"].(bool)
		return v
	}
	switch {
	case len(rest) == 0 && r.Method == http.MethodGet:
		out := []map[string]any{}
		for _, v := range p.Variables {
			out = append(out, asJSON(v))
		}
		writeJSON(w, 200, out)
	case len(rest) == 0 && r.Method == http.MethodPost:
		v := decode()
		if _, ok := p.Variables[v.Key]; ok {
			writeJSON(w, 400, map[string]any{"message": map[string][]string{"key": {"has already been taken"}}})
			return
		}
		if v.Masked && len(v.Value) < 8 {
			writeJSON(w, 400, map[string]any{"message": map[string][]string{"value": {"is invalid"}}})
			return
		}
		p.Variables[v.Key] = v
		writeJSON(w, 201, asJSON(v))
	case len(rest) == 1 && r.Method == http.MethodPut:
		if _, ok := p.Variables[rest[0]]; !ok {
			notFound(w)
			return
		}
		v := decode()
		v.Key = rest[0]
		p.Variables[v.Key] = v
		writeJSON(w, 200, asJSON(v))
	default:
		notFound(w)
	}
}

func (s *Server) serveGeneric(w http.ResponseWriter, r *http.Request, p *Project, key string) {
	switch r.Method {
	case http.MethodPut:
		data, _ := io.ReadAll(r.Body)
		p.Generic[key] = data
		writeJSON(w, 201, map[string]any{"id": len(p.Generic), "file_name": key[strings.LastIndexByte(key, '/')+1:], "size": len(data)})
	case http.MethodHead, http.MethodGet:
		data, ok := p.Generic[key]
		if !ok {
			notFound(w)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(200)
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	default:
		notFound(w)
	}
}
