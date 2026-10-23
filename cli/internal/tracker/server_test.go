package tracker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerOf(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://gitlab.example.com", "gitlab.example.com"},
		{"https://GitLab.Example.COM/", "gitlab.example.com"},
		{"http://gitlab.example.com", "gitlab.example.com"},
		{"gitlab.example.com", "gitlab.example.com"},
		{"  https://gitlab.example.com/group/sub  ", "gitlab.example.com"},
		{"https://gitlab.example.com:443", "gitlab.example.com"},
		{"http://gitlab.example.com:80", "gitlab.example.com"},
		{"https://gitlab.example.com:8443/api", "gitlab.example.com:8443"},
		{"http://127.0.0.1:41234", "127.0.0.1:41234"},
		{"https://user:pw@gitlab.example.com", "gitlab.example.com"},
		{"https://gitlab.example.com.", "gitlab.example.com"},
		{"git@gitlab.example.com:acme/core.git", "gitlab.example.com"},
		{"ssh://git@gitlab.example.com:2222/acme/core.git", "gitlab.example.com"},
		{"https://gitlab.example.com/acme/core.git", "gitlab.example.com"},
		{"gitlab.example.com:8443", "gitlab.example.com:8443"},
		{"", ""},
		{"   ", ""},
		{"https://", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, ServerOf(tt.in), tt.in)
	}
}

func TestSameServer(t *testing.T) {
	assert.True(t, SameServer("https://GitLab.Example.com/", "git@gitlab.example.com:a/b.git"))
	assert.True(t, SameServer("http://h.example:80", "https://h.example:443"))
	assert.False(t, SameServer("https://gitlab.octo.example", "https://gitlab.forge.example"))
	assert.False(t, SameServer("https://h.example:8443", "https://h.example"))
	assert.False(t, SameServer("", ""), "two undeclared servers are not the same server")
}

func TestValidateServer(t *testing.T) {
	assert.NoError(t, ValidateServer(""))
	assert.NoError(t, ValidateServer("https://gitlab.example.com"))
	assert.NoError(t, ValidateServer("gitlab.example.com"))
	assert.Error(t, ValidateServer("https://"))
}
