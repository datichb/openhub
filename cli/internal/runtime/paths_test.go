package ohruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathMap(t *testing.T) {
	m := PathMap{
		{Host: "/Users/me/.oh/bundles/abc", Inner: "/opt/oh/bundle"},
		{Host: "/Users/me/src/app", Inner: "/work/app"},
		{Host: "/Users/me/src/app/vendor/lib", Inner: "/work/lib"},
	}
	cases := map[string]string{
		"/Users/me/src/app":                "/work/app",
		"/Users/me/src/app/":               "/work/app",
		"/Users/me/src/app/main.go":        "/work/app/main.go",
		"/Users/me/src/app/vendor/lib/x.c": "/work/lib/x.c",
		"/Users/me/.oh/bundles/abc/skills": "/opt/oh/bundle/skills",
	}
	for host, inner := range cases {
		got, ok := m.ToInner(host)
		assert.True(t, ok, host)
		assert.Equal(t, inner, got, host)
	}
	_, ok := m.ToInner("/Users/me/src/application")
	assert.False(t, ok, "prefix must stop at a path separator")
	assert.False(t, m.Covers("/etc"))

	back, ok := m.ToHost("/work/app/cmd/main.go")
	assert.True(t, ok)
	assert.Equal(t, "/Users/me/src/app/cmd/main.go", back)
	_, ok = m.ToHost("/tmp/x")
	assert.False(t, ok)
}

func TestPathMapNilIsIdentity(t *testing.T) {
	var m PathMap
	got, ok := m.ToInner("/a/b")
	assert.True(t, ok)
	assert.Equal(t, "/a/b", got)
	assert.True(t, m.Covers("/anything"))
}
