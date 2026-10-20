package testenv

import (
	"sort"
	"strings"
	"testing"
)

// FakeSecretNames are the credentials a user may have in the environment of
// the shell that starts oh, and that must never reach an agent: the ones oh
// manages itself (they only reach the tool as the session token of the
// credential proxy) and the ones it does not (GEMINI_API_KEY, GH_TOKEN…, held
// back by the allow-list of internal/envfilter), including the keys of the
// tests that call a real model.
var FakeSecretNames = []string{
	"AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
	"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY",
	"GH_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN",
	"OH_EVAL_BEDROCK_TOKEN", "OH_E2E_BEDROCK_TOKEN",
}

// FakeSecrets maps each of FakeSecretNames to a value no real credential
// has and no other value contains: "oh-fake-secret/NAME/end".
func FakeSecrets() map[string]string {
	m := make(map[string]string, len(FakeSecretNames))
	for _, n := range FakeSecretNames {
		m[n] = "oh-fake-secret/" + n + "/end"
	}
	return m
}

// SetFakeSecrets puts FakeSecrets in the environment of the test (restored
// at its end) and returns them. A real credential of the same name, in the
// shell that runs the tests, is hidden meanwhile.
func SetFakeSecrets(t testing.TB) map[string]string {
	t.Helper()
	m := FakeSecrets()
	for k, v := range m {
		t.Setenv(k, v)
	}
	return m
}

// Leaks lists the places (the NAME of the entries of env, "NAME=value") where
// a value of secrets appears, whatever the name: the value passed under
// another name, or inside a longer value (a JSON document, a command line),
// is found too. Empty when nothing leaks. Values are never part of the
// result: a failing test must not print the secret it looks for.
func Leaks(secrets map[string]string, env []string) []string {
	var out []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		for secret, fake := range secrets {
			if strings.Contains(kv, fake) {
				out = append(out, name+" holds the value of "+secret)
			}
		}
	}
	sort.Strings(out)
	return out
}

// LeaksInMap is Leaks for an environment given as a map.
func LeaksInMap(secrets, env map[string]string) []string {
	list := make([]string, 0, len(env))
	for k, v := range env {
		list = append(list, k+"="+v)
	}
	return Leaks(secrets, list)
}

// LeaksIn is Leaks for free text (a command line, a file).
func LeaksIn(secrets map[string]string, text string) []string {
	return Leaks(secrets, []string{"text=" + text})
}
