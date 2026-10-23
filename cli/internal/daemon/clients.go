package daemon

import (
	"net/http"
	"sort"
	"time"
)

func (d *Daemon) handleClients(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, d.attachedClients(r.URL.Query().Get("session")))
}

// attachedClients lists the live attached clients of a session ("" = all),
// most recent heartbeat first.
func (d *Daemon) attachedClients(sessionID string) []AttachedClient {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	type item struct {
		c       AttachedClient
		expires time.Time
	}
	var items []item
	for id, c := range d.clients {
		if now.After(c.expires) {
			delete(d.clients, id)
			continue
		}
		if c.kind != ClientAttach || c.session == "" || (sessionID != "" && c.session != sessionID) {
			continue
		}
		items = append(items, item{AttachedClient{ClientID: id, SessionID: c.session, Where: c.where}, c.expires})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].expires.After(items[j].expires) })
	out := make([]AttachedClient, 0, len(items))
	for _, it := range items {
		out = append(out, it.c)
	}
	return out
}
