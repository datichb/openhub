package gitlab

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateGitLabURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string // "" : accepted
	}{
		{"gitlab.com", "https://gitlab.com", ""},
		{"a company instance", "https://gitlab.mycompany.com", ""},
		{"with a path", "https://gitlab.com/api/v4", ""},
		{"plain HTTP", "http://gitlab.com", "must use https"},
		{"empty (parsed, but no scheme)", "", "must use https"},
		{"private address", "https://192.168.1.1", "private/internal address"},
		{"loopback", "https://127.0.0.1", "private/internal address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitLabURL(tt.url)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestIsPrivateHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"8.8.8.8", false},
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"::1", true},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			assert.Equal(t, tt.want, isPrivateHost(tt.host))
		})
	}
}
