package gitutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasTestFiles(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{"nil slice", nil, false},
		{"empty slice", []string{}, false},
		{"Go _test.go", []string{"handler_test.go"}, true},
		{"Go _test.go with path", []string{"pkg/auth/handler_test.go"}, true},
		{"Go _test.go uppercase", []string{"Handler_TEST.go"}, true},
		{"JS/TS .test.tsx", []string{"component.test.tsx"}, true},
		{"JS .spec.js", []string{"component.spec.js"}, true},
		{"Python test_ prefix", []string{"test_utils.py"}, true},
		{"Python _test suffix", []string{"utils_test.py"}, true},
		{"Java Test suffix", []string{"UserServiceTest.java"}, true},
		{"Java Tests suffix", []string{"UserServiceTests.java"}, true},
		{"Ruby _spec.rb", []string{"model_spec.rb"}, true},
		{"Ruby _test.rb", []string{"model_test.rb"}, true},
		{"no test files", []string{"main.go", "utils.go"}, false},
		{"test.go is not _test.go", []string{"test.go"}, false},
		{"mixed with one test", []string{"main.go", "handler_test.go"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasTestFiles(tt.files)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"nil", nil, false},
		{"empty", []byte{}, false},
		{"plain text", []byte("hello world"), false},
		{"null byte in text", []byte{0x48, 0x65, 0x6c, 0x00}, true},
		{"null at position 8191 (inside window)", func() []byte {
			b := make([]byte, 8192)
			for i := range b {
				b[i] = 'A'
			}
			b[8191] = 0x00
			return b
		}(), true},
		{"null at position 8192 (beyond window)", func() []byte {
			b := make([]byte, 8193)
			for i := range b {
				b[i] = 'A'
			}
			b[8192] = 0x00
			return b
		}(), false},
		{"UTF-8 BOM", []byte{0xEF, 0xBB, 0xBF}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinary(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectBaseBranch_ConfiguredOverride(t *testing.T) {
	tests := []struct {
		name           string
		dir            string
		configuredBase string
		want           string
	}{
		{"configured develop", "/tmp", "develop", "develop"},
		{"configured main", "/tmp", "main", "main"},
		{"empty dir with configured", "", "custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectBaseBranch(tt.dir, tt.configuredBase)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ── ReadFileLines ─────────────────────────────────────────────────────────────

func TestReadFileLines(t *testing.T) {
	tooMany := make([]string, maxTotalFiles+1)
	for i := range tooMany {
		tooMany[i] = "file.txt"
	}
	tests := []struct {
		name    string
		files   map[string][]byte
		list    []string
		want    []string
		wantErr string
	}{
		{"files are read in the order of the list", map[string][]byte{"a.go": []byte("package main\nfunc main() {}\n"), "b.txt": []byte("hello\nworld\n")},
			[]string{"a.go", "b.txt"}, []string{"package main", "func main() {}", "hello", "world"}, ""},
		{"binary files are skipped", map[string][]byte{"img.png": {0x89, 0x50, 0x4E, 0x47, 0x00, 0x00}, "ok.txt": []byte("text\n")},
			[]string{"img.png", "ok.txt"}, []string{"text"}, ""},
		{"missing files are skipped", map[string][]byte{"exists.txt": []byte("yes\n")},
			[]string{"gone.txt", "exists.txt"}, []string{"yes"}, ""},
		{"an empty list reads nothing", nil, nil, nil, ""},
		{"too many files", nil, tooMany, nil, "too many files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o644))
			}

			lines, err := ReadFileLines(dir, tt.list)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, lines)
		})
	}
}
