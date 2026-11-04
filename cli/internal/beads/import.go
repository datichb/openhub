package beads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// FindByExternalRef returns the Beads ticket that mirrors an external tracker
// ticket (its external_ref is ref, « gitlab-623 »), or nil when the project has
// none. A ticket still open wins over a closed one.
// Runs: bd -C <path> --readonly list --external-ref <ref> --all --json --flat -n 0
func FindByExternalRef(projectPath, ref string) (*Ticket, error) {
	if ref == "" {
		return nil, errors.New("beads: empty external reference")
	}
	found, err := runBdJSON([]string{"-C", projectPath, "--readonly", "list", "--external-ref", ref, "--all", "--json", "--flat", "-n", "0"})
	if err != nil {
		return nil, err
	}
	var best *Ticket
	for i := range found {
		if found[i].ExternalRef != ref {
			continue // the filter is exact; a ticket that is not ours is never taken
		}
		if best == nil || (IsClosedStatus(best.Status) && !IsClosedStatus(found[i].Status)) {
			best = &found[i]
		}
	}
	return best, nil
}

// ImportFromTracker returns the Beads ticket of a ticket of an external
// tracker (provider « gitlab », id « 623 »), and creates it when the project
// has none: calling it twice for the same ticket gives the same Beads ticket
// and creates one. The structured external_ref links the two (ADR-032) and
// the title carries it, « [gitlab-623] Export CSV ».
// Runs: bd -C <path> --readonly list --external-ref <ref> …, then, when there
// is none, bd -C <path> create <title> --external-ref <ref> --json.
func ImportFromTracker(projectPath, provider, trackerID, title, description string) (id string, created bool, err error) {
	ref := BuildExternalRef(provider, trackerID)
	if ref == "" {
		return "", false, errors.New("beads: a provider and an id are needed to link a ticket")
	}
	if err := Available(); err != nil {
		return "", false, err
	}
	existing, err := FindByExternalRef(projectPath, ref)
	if err != nil {
		return "", false, err
	}
	if existing != nil {
		return existing.ID, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), bdTimeout)
	defer cancel()
	out, err := BdCommand(ctx, importCreateArgs(projectPath, ref, title, description)...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", false, fmt.Errorf("bd create: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", false, fmt.Errorf("bd create: %w", err)
	}
	var made struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &made); err != nil || made.ID == "" {
		return "", false, fmt.Errorf("bd create: no ticket id in the answer %q", strings.TrimSpace(string(out)))
	}
	return made.ID, true, nil
}

// importCreateArgs is the bd create of ImportFromTracker.
func importCreateArgs(projectPath, ref, title, description string) []string {
	args := []string{"-C", projectPath, "create", strings.TrimSpace("[" + ref + "] " + strings.TrimSpace(title)), "--external-ref", ref}
	if description = strings.TrimSpace(description); description != "" {
		args = append(args, "--description="+description)
	}
	return append(args, "--json")
}
