package cmd

import (
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testServePort  = 18080
	testServeToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func serveTestHandler() http.Handler {
	return serveGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "secret dashboard")
	}), testServePort, testServeToken)
}

func serveRequest(t *testing.T, host, target string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = host
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	serveTestHandler().ServeHTTP(rec, req)
	return rec
}

func TestServeGuardRefusesForeignHost(t *testing.T) {
	for _, host := range []string{
		"evil.example.com",
		"evil.example.com:18080",
		"127.0.0.1.evil.example.com:18080",
		"localhost.evil.example.com:18080",
		"127.0.0.1:9999",
		"127.0.0.1",
		"localhost",
		"0.0.0.0:18080",
		"192.168.1.10:18080",
		"",
	} {
		// even with a valid token: the name is what a DNS rebinding changes
		rec := serveRequest(t, host, "/api/v1/projects?token="+testServeToken, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "host %q", host)
		assert.NotContains(t, rec.Body.String(), "secret", "host %q", host)
		assert.Empty(t, rec.Result().Cookies(), "host %q", host)
	}
}

func TestServeGuardAcceptsLoopbackHosts(t *testing.T) {
	for _, host := range []string{"127.0.0.1:18080", "localhost:18080", "[::1]:18080", "LOCALHOST:18080"} {
		rec := serveRequest(t, host, "/api/v1/projects?token="+testServeToken, nil)
		assert.Equal(t, http.StatusOK, rec.Code, "host %q", host)
	}
}

func TestServeGuardRefusesMissingOrWrongToken(t *testing.T) {
	cases := map[string]func(*http.Request){
		"no token":           nil,
		"empty token":        func(r *http.Request) { r.URL.RawQuery = "token=" },
		"wrong token":        func(r *http.Request) { r.URL.RawQuery = "token=" + testServeToken[:63] + "0" },
		"truncated token":    func(r *http.Request) { r.URL.RawQuery = "token=" + testServeToken[:10] },
		"longer token":       func(r *http.Request) { r.URL.RawQuery = "token=" + testServeToken + "0" },
		"wrong cookie":       func(r *http.Request) { r.AddCookie(&http.Cookie{Name: serveCookieName(testServePort), Value: "nope"}) },
		"cookie of the port": func(r *http.Request) { r.AddCookie(&http.Cookie{Name: serveCookieName(9999), Value: testServeToken}) },
		"token in a header":  func(r *http.Request) { r.Header.Set("X-Token", testServeToken) },
	}
	for name, mutate := range cases {
		rec := serveRequest(t, "127.0.0.1:18080", "/api/v1/projects", mutate)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, name)
		assert.NotContains(t, rec.Body.String(), "secret", name)
		assert.Empty(t, rec.Result().Cookies(), name)
	}
}

func TestServeGuardValidTokenSetsCookie(t *testing.T) {
	rec := serveRequest(t, "127.0.0.1:18080", "/api/v1/projects?token="+testServeToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "secret dashboard")
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, serveCookieName(testServePort), c.Name)
	assert.Equal(t, testServeToken, c.Value)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
	assert.Equal(t, "/", c.Path)

	// the cookie alone is enough afterwards
	rec = serveRequest(t, "localhost:18080", "/api/v1/projects", func(r *http.Request) { r.AddCookie(c) })
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "secret dashboard")
}

// The printed URL opens the page: the cookie is set and the token leaves the
// address bar (history, Referer).
func TestServeGuardPageRedirectsWithoutToken(t *testing.T) {
	rec := serveRequest(t, "127.0.0.1:18080", "/?token="+testServeToken+"&project=p", nil)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/?project=p", rec.Header().Get("Location"))
	require.Len(t, rec.Result().Cookies(), 1)
	assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
	assert.NotContains(t, rec.Body.String(), "secret")
}

func TestServeGuardCookieIsNotSetOnRefusal(t *testing.T) {
	rec := serveRequest(t, "evil.example.com:18080", "/?token="+testServeToken, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Result().Cookies())
	assert.Empty(t, rec.Header().Get("Location"))
}

func TestNewServeToken(t *testing.T) {
	a, err := newServeToken()
	require.NoError(t, err)
	b, err := newServeToken()
	require.NoError(t, err)
	assert.Regexp(t, `^[0-9a-f]{64}$`, a)
	assert.NotEqual(t, a, b)
}

// End to end on a real socket: a browser keeps the cookie of the printed URL
// and the same server refuses another Host header (DNS rebinding).
func TestServeGuardOverHTTP(t *testing.T) {
	srv := httptest.NewUnstartedServer(nil)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	srv.Config.Handler = serveGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "secret dashboard")
	}), port, testServeToken)
	srv.Start()
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}
	get := func(path, host string) (int, string) {
		req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		require.NoError(t, err)
		if host != "" {
			req.Host = host
		}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		buf := make([]byte, 64)
		n, _ := resp.Body.Read(buf)
		return resp.StatusCode, string(buf[:n])
	}

	code, _ := get("/api/v1/projects", "")
	assert.Equal(t, http.StatusUnauthorized, code, "no token yet")
	code, body := get("/?token="+testServeToken, "")
	assert.Equal(t, http.StatusOK, code, "the redirect to / is followed")
	assert.Contains(t, body, "secret dashboard")
	code, body = get("/api/v1/projects", "")
	assert.Equal(t, http.StatusOK, code, "the cookie is kept")
	assert.Contains(t, body, "secret dashboard")
	code, body = get("/api/v1/projects", fmt.Sprintf("rebind.example.com:%d", port))
	assert.Equal(t, http.StatusForbidden, code, "foreign Host, valid cookie")
	assert.NotContains(t, body, "secret")
}
