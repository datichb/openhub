package selfupdate

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

// fakeGitHub is a TLS server standing for every host: the transport sends all
// connections to it whatever the host of the URL, and handler sees the host
// asked for in r.Host.
type fakeGitHub struct {
	mu   sync.Mutex
	seen []string
}

func (f *fakeGitHub) hits(host string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, h := range f.seen {
		if strings.HasPrefix(h, host+"/") || h == host {
			n++
		}
	}
	return n
}

func startFakeGitHub(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.seen = append(f.seen, r.Host+r.URL.Path)
		f.mu.Unlock()
		handler(w, r)
	}))
	addr := srv.Listener.Addr().String()
	httpTransport = &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test server, self-signed certificate
	}
	t.Cleanup(func() {
		httpTransport = nil
		srv.Close()
	})
	return f
}

func TestCheckRedirect(t *testing.T) {
	ok := []string{
		"https://github.com/datichb/openhub/releases/download/v5.0.0/openhub_linux_amd64.tar.gz",
		"https://api.github.com/repositories/1/releases/latest",
		"https://objects.githubusercontent.com/github-production-release-asset/1/abc?X-Amz=1",
		"https://release-assets.githubusercontent.com/github-production-release-asset/1/abc",
		"https://github-releases.githubusercontent.com/1/abc",
		"https://GitHub.com:443/x",
	}
	for _, u := range ok {
		req, _ := http.NewRequest(http.MethodGet, u, http.NoBody)
		if err := checkRedirect(req, nil); err != nil {
			t.Errorf("redirect to %q refused: %v", u, err)
		}
	}
	refused := []string{
		"http://github.com/x",
		"ftp://github.com/x",
		"https://evil.example/x",
		"https://github.com.evil.example/x",
		"https://evilgithub.com/x",
		"https://github.com@evil.example/x",
		"https://raw.githubusercontent.com/x",
		"https://sub.github.com/x",
		"https://github.com:8443/x",
		"https://127.0.0.1/x",
		"https://localhost/x",
	}
	for _, u := range refused {
		req, err := http.NewRequest(http.MethodGet, u, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkRedirect(req, nil); !errors.Is(err, ErrRedirect) {
			t.Errorf("redirect to %q: got %v, want ErrRedirect", u, err)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://github.com/x", http.NoBody)
	if err := checkRedirect(req, make([]*http.Request, maxRedirects)); !errors.Is(err, ErrRedirect) {
		t.Errorf("too many redirects: got %v", err)
	}
}

func redirectTo(target string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}
}

// S-39: a redirect to a host that is not GitHub's is refused, once (it is
// not retried) and without a single request to that host.
func TestFetchReleaseRefusesRedirectToUnknownHost(t *testing.T) {
	f := startFakeGitHub(t, redirectTo("https://evil.example/steal"))
	_, err := fetchRelease("https://api.github.com/repos/datichb/openhub/releases/latest")
	if !errors.Is(err, ErrRedirect) {
		t.Fatalf("got %v, want ErrRedirect", err)
	}
	if n := f.hits("evil.example"); n != 0 {
		t.Errorf("evil.example was requested %d time(s)", n)
	}
	if n := f.hits("api.github.com"); n != 1 {
		t.Errorf("api.github.com requested %d times, want 1 (a refused redirect is not retried)", n)
	}
}

func TestDownloadRefusesRedirects(t *testing.T) {
	for name, target := range map[string]string{
		"unknown host":  "https://evil.example/openhub.tar.gz",
		"plain http":    "http://github.com/openhub.tar.gz",
		"lookalike":     "https://github.com.evil.example/openhub.tar.gz",
		"odd port":      "https://objects.githubusercontent.com:8443/openhub.tar.gz",
		"scheme change": "ftp://objects.githubusercontent.com/openhub.tar.gz",
	} {
		t.Run(name, func(t *testing.T) {
			f := startFakeGitHub(t, redirectTo(target))
			dest, err := os.CreateTemp(t.TempDir(), "dl")
			if err != nil {
				t.Fatal(err)
			}
			defer dest.Close()
			err = downloadAsset(&Asset{Name: "openhub_linux_amd64.tar.gz", BrowserDownloadURL: "https://github.com/datichb/openhub/releases/download/v9.9.9/openhub_linux_amd64.tar.gz"}, dest, nil)
			if !errors.Is(err, ErrRedirect) {
				t.Fatalf("got %v, want ErrRedirect", err)
			}
			if n := f.hits("github.com"); n != 1 {
				t.Errorf("github.com requested %d times, want 1", n)
			}
			if st, _ := dest.Stat(); st.Size() != 0 {
				t.Errorf("%d byte(s) written", st.Size())
			}
		})
	}
}

func TestFetchChecksumsRefusesRedirectToUnknownHost(t *testing.T) {
	startFakeGitHub(t, redirectTo("https://evil.example/checksums.txt"))
	_, err := fetchChecksums(&Asset{Name: "checksums.txt", BrowserDownloadURL: "https://github.com/datichb/openhub/releases/download/v9.9.9/checksums.txt"})
	if !errors.Is(err, ErrRedirect) {
		t.Fatalf("got %v, want ErrRedirect", err)
	}
}

func TestDownloadRedirectLoopStops(t *testing.T) {
	f := startFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/again", http.StatusFound)
	})
	dest, err := os.CreateTemp(t.TempDir(), "dl")
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()
	err = downloadAsset(&Asset{Name: "a", BrowserDownloadURL: "https://github.com/loop"}, dest, nil)
	if !errors.Is(err, ErrRedirect) {
		t.Fatalf("got %v, want ErrRedirect", err)
	}
	if n := f.hits("github.com"); n > maxRedirects+1 {
		t.Errorf("%d requests for a loop", n)
	}
}

// The real download path: github.com redirects to the CDN (objects.* or
// release-assets.*), which stays allowed.
func TestDownloadFollowsRedirectToReleaseCDN(t *testing.T) {
	for _, cdn := range []string{"objects.githubusercontent.com", "release-assets.githubusercontent.com"} {
		t.Run(cdn, func(t *testing.T) {
			startFakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Host == "github.com" {
					http.Redirect(w, r, "https://"+cdn+"/signed/asset?X-Amz-Signature=abc", http.StatusFound)
					return
				}
				_, _ = w.Write([]byte("archive-bytes"))
			})
			dest, err := os.CreateTemp(t.TempDir(), "dl")
			if err != nil {
				t.Fatal(err)
			}
			defer dest.Close()
			if err := downloadAsset(&Asset{Name: "a", BrowserDownloadURL: "https://github.com/datichb/openhub/releases/download/v9.9.9/a"}, dest, nil); err != nil {
				t.Fatalf("download through %s: %v", cdn, err)
			}
			data, _ := os.ReadFile(dest.Name())
			if string(data) != "archive-bytes" {
				t.Errorf("downloaded %q", data)
			}
		})
	}
}
