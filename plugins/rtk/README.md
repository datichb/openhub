# RTK Plugin for OpenCode

Automatically rewrites bash commands to use [RTK (Rust Token Killer)](https://www.rtk-ai.app/) for 60-90% token savings on CLI output.

## Features

- ✅ **Automatic Command Rewriting** — All bash commands are rewritten to use RTK when possible
- ✅ **Project-Scoped Tracking** — Isolated stats per OpenCode project
- ✅ **Visual Feedback** — Toast notifications for commands saving >10K tokens
- ✅ **Session Summary** — Automatic report on session completion
- ✅ **Detailed Logging** — Structured logs for monitoring and debugging

## Requirements

- **RTK** >= 0.42.0 — installed automatically by `oc plugin install rtk`, or manually:
  - macOS: `brew install rtk`
  - Rust/Cargo: `cargo install rtk`
  - Other: [rtk-ai.app](https://www.rtk-ai.app/)
- **OpenCode** >= 1.15.0 with plugin support

## Installation

### Automatic (via opencode-hub)

```bash
cd ~/.opencode-hub
oc plugin install rtk
```

If RTK is not yet installed, the script will offer to install it automatically.
You can also install RTK manually beforehand:
- macOS: `brew install rtk`
- Rust/Cargo: `cargo install rtk`
- Other: [rtk-ai.app](https://www.rtk-ai.app/)

### Manual

```bash
# Copy plugin to OpenCode config directory
cp rtk.ts ~/.config/opencode/plugins/rtk.ts

# Restart OpenCode if running
```

## Verification

### Check Plugin is Loaded

```bash
# Launch OpenCode
opencode

# Check logs for initialization message
tail -f ~/.cache/opencode/logs/opencode.log | grep rtk-plugin
# Should see: "RTK plugin initialized"
```

### Test Token Savings

```bash
# In OpenCode, run a command that generates lots of output:
> Run: rtk git diff HEAD~10 HEAD

# You should see:
# 1. Command executes with filtered output
# 2. Toast notification if >10K tokens saved
# 3. Logs showing the savings
```

## Configuration

### Adjust Notification Threshold

By default, toasts appear for commands saving >10K tokens. To change this:

1. Edit `~/.config/opencode/plugins/rtk.ts`
2. Find line 208: `if (estimatedCommandSaving > 10000) {`
3. Change `10000` to your preferred threshold:
   - `5000` — Frequent notifications
   - `20000` — Conservative (recommended for noisy environments)
   - `50000` — Only huge savings

### Disable Toast Notifications

Comment out the toast section (lines 209-214) to keep only logs.

## Monitoring

### Real-time Logs

```bash
tail -f ~/.cache/opencode/logs/opencode.log | grep rtk-plugin
```

### Session Stats

After each OpenCode session, check the summary in logs or wait for the idle toast.

### Project Stats

```bash
# From your project directory
rtk gain --project
rtk gain --project --daily
```

### Global Stats

```bash
rtk gain
rtk gain --history
rtk gain --graph
```

## Troubleshooting

### Plugin Not Loading

**Symptom:** No "RTK plugin initialized" in logs

**Solution:**
```bash
# Check RTK is installed
which rtk
rtk --version  # Should be >= 0.42.0

# Check plugin file exists
ls -la ~/.config/opencode/plugins/rtk.ts

# Check for errors in OpenCode logs
grep "rtk-plugin" ~/.cache/opencode/logs/opencode.log
```

### Commands Not Being Rewritten

**Symptom:** Commands run without RTK prefix

**Possible causes:**
1. Command already starts with `rtk` (no rewrite needed)
2. Command is not supported by RTK (e.g., `cd`, `export`)
3. RTK version is too old

**Debug:**
```bash
# Test if a command can be rewritten
rtk hook check "git diff HEAD~5 HEAD"
# Should output: rtk git diff HEAD~5 HEAD

# Update RTK
brew upgrade rtk
# or with Rust/Cargo:
cargo install rtk

# Check logs for "not rewritable" messages
grep "not rewritable" ~/.cache/opencode/logs/opencode.log
```

### No Toast Notifications

**Possible causes:**
1. Savings are below threshold (default 10K tokens)
2. Desktop app notifications disabled
3. Session hasn't completed yet (summary toast)

**Solution:**
- Lower threshold (see Configuration above)
- Check system notification settings
- Wait for session idle or check logs directly

### Incorrect Savings Estimates

**Note:** Per-command savings are estimated by dividing session total by command count. This is approximate.

For exact per-command savings:
```bash
rtk gain --history
```

## Version History

- **1.0.0** (2026-05-29) — Initial release with RTK 0.42.0 support

## Contributing

This plugin is part of [opencode-hub](https://github.com/datichb/opencode-hub).

Issues and PRs welcome!

## License

MIT
