package termlaunch

import "fmt"

// openAppleTerminal opens a new tab in Terminal.app, cds into dir, and runs command.
func openAppleTerminal(dir, command string) error {
	// Escape single quotes in dir and command for AppleScript string literals.
	safeDir := applescriptEscape(dir)
	safeCmd := applescriptEscape(command)

	script := fmt.Sprintf(`
tell application "Terminal"
	activate
	do script "cd '%s' && %s"
end tell`, safeDir, safeCmd)

	return runOsascript(script)
}

// openITerm opens a new tab in iTerm2, cds into dir, and runs command.
func openITerm(dir, command string) error {
	safeDir := applescriptEscape(dir)
	safeCmd := applescriptEscape(command)

	script := fmt.Sprintf(`
tell application "iTerm2"
	activate
	tell current window
		create tab with default profile
		tell current session of current tab
			write text "cd '%s' && %s"
		end tell
	end tell
end tell`, safeDir, safeCmd)

	return runOsascript(script)
}

// applescriptEscape escapes single quotes for use inside AppleScript string literals.
func applescriptEscape(s string) string {
	// In AppleScript strings delimited by single quotes, escape ' as '\''
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\\', '\'', '\'')
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
}
