// Package envfilter decides which variables of the user's environment reach
// the tool server of a local session, and through it every shell an agent
// runs (audit v5, S-28).
//
// The filter is an allow-list. A deny-list always misses a secret (the
// GEMINI_API_KEY, GH_TOKEN, NPM_TOKEN… of the day): what the tool and its
// processes need is listed, the rest does not pass. The user adds variables
// with [execution] env_allow (names, or PREFIX_* patterns).
//
// Containers and remote jobs do not use it: they build their environment
// from nothing (oh's settings only).
package envfilter

import (
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// baseNames are the variables that always pass: what a process needs to run
// (identity, terminal, language, temporary files), the network (proxy,
// certificates), the locations of the toolchains the agents build with
// (paths, never credentials), and the settings oh reads in the processes it
// starts under the server (oh mcp…).
//
// SSH_AUTH_SOCK is not listed: it hands the keys of the ssh-agent of the user
// to the agents. Add it with env_allow if their git runs over ssh.
var baseNames = []string{
	// Process, identity, terminal, language, temporary files.
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TERM", "COLORTERM",
	"LANG", "LANGUAGE", "TZ", "TMPDIR", "TMP", "TEMP",
	"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "FORCE_COLOR",
	// Network: proxies and certificate authorities.
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY",
	"http_proxy", "https_proxy", "no_proxy", "all_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS",
	"REQUESTS_CA_BUNDLE", "CURL_CA_BUNDLE", "GIT_SSL_CAINFO",
	// Toolchains: where they are installed.
	"GOPATH", "GOROOT", "GOCACHE", "GOMODCACHE", "GOBIN",
	"JAVA_HOME", "MAVEN_HOME", "M2_HOME", "GRADLE_HOME", "GRADLE_USER_HOME",
	"CARGO_HOME", "RUSTUP_HOME", "NVM_DIR", "PNPM_HOME", "VOLTA_HOME", "BUN_INSTALL",
	"PYENV_ROOT", "VIRTUAL_ENV", "CONDA_PREFIX", "RBENV_ROOT", "GEM_HOME", "GEM_PATH",
	"ASDF_DIR", "ASDF_DATA_DIR", "DOTNET_ROOT", "ANDROID_HOME", "ANDROID_SDK_ROOT", "HOMEBREW_PREFIX",
	// oh: read by the processes it starts under the server.
	"OH_HOME", "OH_WORKFLOWS_DIR", "OH_WORKFLOW_REPORTS", "OH_ALLOW_NEWER_SCHEMA", "OH_SKIP_DB_BACKUP", "OH_DAEMON_INPROCESS",
	// Windows (names are compared without case there).
	"SystemRoot", "SystemDrive", "windir", "ComSpec", "PATHEXT", "USERPROFILE", "USERNAME", "USERDOMAIN",
	"APPDATA", "LOCALAPPDATA", "ProgramFiles", "ProgramFiles(x86)", "ProgramW6432", "ProgramData",
	"CommonProgramFiles", "CommonProgramFiles(x86)", "CommonProgramW6432", "HOMEDRIVE", "HOMEPATH",
	"COMPUTERNAME", "OS", "NUMBER_OF_PROCESSORS", "PROCESSOR_ARCHITECTURE", "PROCESSOR_IDENTIFIER",
	"ALLUSERSPROFILE", "PUBLIC", "SESSIONNAME",
}

// basePrefixes are the families that always pass (LC_ALL, LC_CTYPE…, XDG_*).
var basePrefixes = []string{"LC_", "XDG_"}

// hardDenied are the credentials oh manages itself: they reach the tool only
// as the session token of the credential proxy, whatever env_allow says.
var hardDenied = map[string]bool{
	"AWS_BEARER_TOKEN_BEDROCK": true,
	"AWS_ACCESS_KEY_ID":        true,
	"AWS_SECRET_ACCESS_KEY":    true,
	"AWS_SESSION_TOKEN":        true,
	"AWS_PROFILE":              true,
	"ANTHROPIC_API_KEY":        true,
	"OPENAI_API_KEY":           true,
	"OPENROUTER_API_KEY":       true,
	"GITLAB_TOKEN":             true,
	"GITHUB_TOKEN":             true,
	"FIGMA_TOKEN":              true,
	"JIRA_TOKEN":               true,
	"LINEAR_API_KEY":           true,
	"GOOGLE_ACCESS_TOKEN":      true,
	"OH_PASSPHRASE":            true,
}

// foldCase: environment variable names are not case sensitive on Windows.
var foldCase = runtime.GOOS == "windows"

func sameName(a, b string) bool {
	if foldCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func hasPrefix(name, prefix string) bool {
	if foldCase {
		return len(name) >= len(prefix) && strings.EqualFold(name[:len(prefix)], prefix)
	}
	return strings.HasPrefix(name, prefix)
}

// HardDenied reports whether oh removes a variable even when env_allow
// lists it: the credentials of the providers and services oh manages.
func HardDenied(name string) bool {
	if foldCase {
		name = strings.ToUpper(name)
	}
	return hardDenied[name]
}

// BaseNames returns the names and the prefixes (ending with "_*") that
// always pass, for the documentation and oh doctor.
func BaseNames() []string {
	out := append([]string(nil), baseNames...)
	for _, p := range basePrefixes {
		out = append(out, p+"*")
	}
	sort.Strings(out)
	return out
}

// patternRe is a variable name, or a prefix ending with a star.
var patternRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_()]*\*?$`)

// ValidPattern reports whether p is an env_allow entry: a variable name, or
// PREFIX_* (a prefix of at least two characters before the star: "*" alone
// would switch the filter off).
func ValidPattern(p string) bool {
	if !patternRe.MatchString(p) {
		return false
	}
	return !strings.HasSuffix(p, "*") || len(p) >= 3
}

func matchPattern(name, pattern string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return hasPrefix(name, prefix)
	}
	return sameName(name, pattern)
}

// FromBase reports whether the base list lets a variable through.
func FromBase(name string) bool {
	for _, n := range baseNames {
		if sameName(name, n) {
			return true
		}
	}
	for _, p := range basePrefixes {
		if hasPrefix(name, p) {
			return true
		}
	}
	return false
}

// FromExtra reports whether one of the env_allow entries lets a variable
// through. Invalid entries match nothing.
func FromExtra(name string, extra []string) bool {
	for _, p := range extra {
		if ValidPattern(p) && matchPattern(name, p) {
			return true
		}
	}
	return false
}

// Allowed reports whether a variable reaches the tool server: in the base
// list or in env_allow, and not one of the credentials oh manages.
func Allowed(name string, extra []string) bool {
	if name == "" || HardDenied(name) {
		return false
	}
	return FromBase(name) || FromExtra(name, extra)
}

// Split sorts an environment ("NAME=value" entries) into the entries that
// pass and the names that do not (never their values).
func Split(environ, extra []string) (kept, dropped []string) {
	for _, kv := range environ {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || name == "" {
			continue // malformed, or a Windows drive entry ("=C:=…")
		}
		if Allowed(name, extra) {
			kept = append(kept, kv)
		} else {
			dropped = append(dropped, name)
		}
	}
	return kept, dropped
}

// secretMarks end the names of variables that hold a secret.
var secretMarks = []string{"_KEY", "_TOKEN", "_SECRET", "_PASSWORD", "_PASSWD", "_PASSPHRASE", "_CREDENTIAL", "_CREDENTIALS"}

// LooksSecret reports whether a name looks like one of a secret
// (*_KEY, *_TOKEN, *_SECRET, *_PASSWORD…, or TOKEN, SECRET, PASSWORD alone).
// A trailing star of a pattern is ignored.
func LooksSecret(name string) bool {
	up := strings.ToUpper(strings.TrimSuffix(name, "*"))
	for _, m := range secretMarks {
		if strings.HasSuffix(up, m) || up == m[1:] {
			return true
		}
	}
	return false
}
