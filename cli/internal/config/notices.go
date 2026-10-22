package config

import (
	"fmt"
	"sync"
)

// NoticeKind tells what a Notice is about.
type NoticeKind string

const (
	// NoticeLegacyKey: a former key of hub.toml was read (legacy_keys.go).
	NoticeLegacyKey NoticeKind = "legacy_key"
	// NoticeLegacyKeychain: a token was read under its former keychain key
	// name (mcpkeys.go).
	NoticeLegacyKeychain NoticeKind = "legacy_keychain"
	// NoticeUnknownKey: a key of hub.toml that no setting uses; ignored.
	NoticeUnknownKey NoticeKind = "unknown_key"
)

// LegacySupportEnds is the version that stops reading the former keys.
const LegacySupportEnds = "v6"

// Notice is something worth telling the user about the files read: a former
// key still accepted, a key ignored.
type Notice struct {
	Kind NoticeKind
	// Key is the key as the file writes it (dotted) or the former keychain
	// key name.
	Key string
	// Replacement is the current key or keychain key name, when there is one.
	Replacement string
	// File, Line and Column locate a key of hub.toml (0 when unknown).
	File         string
	Line, Column int
}

func (n Notice) id() string { return string(n.Kind) + "\x00" + n.Key }

// String is the English text of the notice (the command line shows its own,
// translated).
func (n Notice) String() string {
	where := ""
	if n.Line > 0 {
		where = fmt.Sprintf(" (%s:%d:%d)", n.File, n.Line, n.Column)
	}
	switch n.Kind {
	case NoticeLegacyKey:
		return fmt.Sprintf("former key %s%s: use %s; support ends in %s", n.Key, where, n.Replacement, LegacySupportEnds)
	case NoticeLegacyKeychain:
		return fmt.Sprintf("token read under the former keychain key %s: use %s; support ends in %s", n.Key, n.Replacement, LegacySupportEnds)
	default:
		return fmt.Sprintf("unknown key %s%s ignored", n.Key, where)
	}
}

var notices struct {
	sync.Mutex
	seen    map[string]bool
	pending []Notice
	handler func(Notice)
}

// SetNoticeHandler sets the function that receives the notices, each one a
// single time per process (the same key read again is not repeated); the
// notices raised before are delivered now. A nil handler holds them back.
// The handler runs where the notice is raised, possibly under the lock of
// Load: it must not call Load.
func SetNoticeHandler(h func(Notice)) {
	notices.Lock()
	notices.handler = h
	pending := notices.pending
	if h != nil {
		notices.pending = nil
	}
	notices.Unlock()
	if h == nil {
		return
	}
	for _, n := range pending {
		h(n)
	}
}

// notify records a notice and hands it to the handler, once per process.
func notify(n Notice) {
	notices.Lock()
	if notices.seen == nil {
		notices.seen = map[string]bool{}
	}
	if notices.seen[n.id()] {
		notices.Unlock()
		return
	}
	notices.seen[n.id()] = true
	h := notices.handler
	if h == nil {
		notices.pending = append(notices.pending, n)
	}
	notices.Unlock()
	if h != nil {
		h(n)
	}
}

// ResetNotices forgets the notices already raised (tests).
func ResetNotices() {
	notices.Lock()
	defer notices.Unlock()
	notices.seen = nil
	notices.pending = nil
	notices.handler = nil
}
