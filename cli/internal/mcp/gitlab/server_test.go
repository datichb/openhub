package gitlab

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateGitLabURL_Valid(t *testing.T) {
	err := validateGitLabURL("https://gitlab.com")
	assert.NoError(t, err)
}

func TestValidateGitLabURL_Custom(t *testing.T) {
	err := validateGitLabURL("https://gitlab.mycompany.com")
	assert.NoError(t, err)
}

func TestValidateGitLabURL_HTTP(t *testing.T) {
	err := validateGitLabURL("http://gitlab.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must use https")
}

func TestValidateGitLabURL_PrivateIP(t *testing.T) {
	err := validateGitLabURL("https://192.168.1.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "private/internal address")
}

func TestValidateGitLabURL_Loopback(t *testing.T) {
	err := validateGitLabURL("https://127.0.0.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "private/internal address")
}

func TestValidateGitLabURL_Empty(t *testing.T) {
	// Empty string: url.Parse("") succeeds but scheme is empty
	err := validateGitLabURL("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must use https")
}

func TestIsPrivateHost_PublicIP(t *testing.T) {
	assert.False(t, isPrivateHost("8.8.8.8"))
}

func TestIsPrivateHost_Loopback(t *testing.T) {
	assert.True(t, isPrivateHost("127.0.0.1"))
}

func TestIsPrivateHost_RFC1918(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			assert.Equal(t, tt.want, isPrivateHost(tt.ip))
		})
	}
}

func TestIsPrivateHost_LinkLocal(t *testing.T) {
	assert.True(t, isPrivateHost("169.254.1.1"))
}

func TestIsPrivateHost_IPv6Loopback(t *testing.T) {
	assert.True(t, isPrivateHost("::1"))
}

func TestValidateGitLabURL_WithPath(t *testing.T) {
	// Valid URL with a path component
	err := validateGitLabURL("https://gitlab.com/api/v4")
	assert.NoError(t, err)
}
