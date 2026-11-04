package teamstate

import (
	"log/slog"
	"path/filepath"
	"sync"
)

// MappingNotice is what reading the config.toml of a team clone has to say
// about its tracker mappings: how many values are read as the status they
// stand for and how many stand for nothing. The handler turns it into one
// sentence for the user; the detail of each value is in the log.
type MappingNotice struct {
	// Team is the name of the clone (the last element of its path).
	Team string
	// Path is the local clone.
	Path string
	// Migrated is the number of values read as a status of the board.
	Migrated int
	// Invalid is the number of values that stand for nothing.
	Invalid int
	// Example is the first value that is read as a status (zero when
	// Migrated is 0).
	Example MappingIssue
}

var mappingNotices struct {
	sync.Mutex
	seen    map[string]bool
	handler func(MappingNotice)
}

// SetMappingNoticeHandler sets the function that receives the notices, one
// per team clone and per process (a config is read many times a minute by the
// board). The handler runs where the config is read, possibly under the lock
// of the clone: it must not read the clone.
func SetMappingNoticeHandler(h func(MappingNotice)) {
	mappingNotices.Lock()
	mappingNotices.handler = h
	mappingNotices.Unlock()
}

// noticeMappings logs every value that is not a status of the board (Info:
// the log of oh -v) and hands one notice for the clone to the handler, the
// first time only.
func noticeMappings(repoPath string, issues []MappingIssue) {
	if len(issues) == 0 {
		return
	}
	mappingNotices.Lock()
	if mappingNotices.seen == nil {
		mappingNotices.seen = map[string]bool{}
	}
	if mappingNotices.seen[repoPath] {
		mappingNotices.Unlock()
		return
	}
	mappingNotices.seen[repoPath] = true
	h := mappingNotices.handler
	mappingNotices.Unlock()

	n := MappingNotice{Team: filepath.Base(repoPath), Path: repoPath}
	for _, issue := range issues {
		if issue.Migrated() {
			slog.Info("teamstate.config.mapping_migrated", "table", issue.Table, "key", issue.Key, "value", issue.Value, "used", issue.Status, "config", repoPath)
			if n.Migrated == 0 {
				n.Example = issue
			}
			n.Migrated++
			continue
		}
		slog.Info("teamstate.config.mapping_invalid", "table", issue.Table, "key", issue.Key, "value", issue.Value, "config", repoPath)
		n.Invalid++
	}
	if h != nil {
		h(n)
	}
}
