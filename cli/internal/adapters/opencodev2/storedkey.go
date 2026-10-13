package opencodev2

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"

	"github.com/datichb/openhub/cli/internal/adapters"
)

var _ adapters.StoredKeyReader = (*Adapter)(nil)

// StoredProviderKey implements adapters.StoredKeyReader: the API key of a
// provider stored by `opencode auth` (read with `opencode auth export`).
func (a *Adapter) StoredProviderKey(ctx context.Context, provider string) (string, error) {
	bin := a.Binary
	if bin == "" {
		bin = Command
	}
	out, err := exec.CommandContext(ctx, bin, "auth", "export").Output()
	if err != nil {
		return "", err
	}
	return storedKey(out, provider)
}

func storedKey(export []byte, provider string) (string, error) {
	var creds []struct {
		IntegrationID string `json:"integrationID"`
		Value         struct {
			Key string `json:"key"`
		} `json:"value"`
	}
	if err := json.Unmarshal(export, &creds); err != nil {
		return "", errors.New("unreadable credential export")
	}
	for _, c := range creds {
		if c.IntegrationID == provider && c.Value.Key != "" {
			return c.Value.Key, nil
		}
	}
	return "", nil
}
