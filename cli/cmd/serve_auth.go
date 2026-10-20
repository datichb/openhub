package cmd

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// serveTokenQuery is the query parameter that carries the access token in the
// URL printed by `oh serve`.
const serveTokenQuery = "token"

// newServeToken returns a random 256-bit access token (hex) for one run of
// `oh serve` (S-36).
func newServeToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("%s: %w", i18n.T("cmd.serve.err.token_gen"), err)
	}
	return hex.EncodeToString(b), nil
}

// serveCookieName is per port: cookies do not isolate ports, so two
// `oh serve` (or any other local service) must not overwrite each other's.
func serveCookieName(port int) string {
	return fmt.Sprintf("oh_serve_%d", port)
}

// serveAllowedHost reports whether host (the Host header) is one of the
// loopback names of the dashboard. Anything else is a request that reached
// the local server through another name, typically a DNS rebinding (S-36).
func serveAllowedHost(host string, port int) bool {
	host = strings.ToLower(host)
	for _, name := range []string{"127.0.0.1", "localhost", "[::1]"} {
		if host == fmt.Sprintf("%s:%d", name, port) {
			return true
		}
	}
	return false
}

// serveTokenEqual compares tokens in constant time.
func serveTokenEqual(got, want string) bool {
	return len(got) == len(want) && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// serveGuard protects the dashboard (S-36):
//   - a request whose Host header is not 127.0.0.1:<port>, localhost:<port> or
//     [::1]:<port> is refused (403): a web page cannot read the dashboard by
//     DNS rebinding;
//   - a request without the access token is refused (401). The token comes in
//     the `token` query parameter (the printed URL) or in the cookie set
//     right after: HttpOnly, SameSite=Strict, named after the port. The page
//     itself is then served without the token in its URL (303).
func serveGuard(next http.Handler, port int, token string) http.Handler {
	cookie := serveCookieName(port)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !serveAllowedHost(r.Host, port) {
			http.Error(w, i18n.T("cmd.serve.err.host"), http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")

		q := r.URL.Query()
		if got := q.Get(serveTokenQuery); got != "" && serveTokenEqual(got, token) {
			http.SetCookie(w, &http.Cookie{
				Name:     cookie,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			if r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				q.Del(serveTokenQuery)
				target := url.URL{Path: "/", RawQuery: q.Encode()}
				http.Redirect(w, r, target.String(), http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if c, err := r.Cookie(cookie); err == nil && serveTokenEqual(c.Value, token) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, i18n.T("cmd.serve.err.token"), http.StatusUnauthorized)
	})
}
