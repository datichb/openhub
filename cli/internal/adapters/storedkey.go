package adapters

import "context"

// StoredKeyReader is implemented by adapters whose tool keeps provider API
// keys of its own (evaluations without an oh secret store): it returns the
// key stored for a tool provider id ("" when none).
type StoredKeyReader interface {
	StoredProviderKey(ctx context.Context, provider string) (string, error)
}
