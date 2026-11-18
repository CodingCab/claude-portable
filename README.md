# claude-portable

A tiny cross-platform launcher (~3 MB) that runs Claude Code anywhere — no
install, no admin rights, no Node.js required.

On first run it downloads the official Anthropic Claude Code binary for your
platform (~380 MB, one time). Every run after that is instant.

## Usage

```
claude-portable [options] [claude args...]
```

Drop `claude-portable` (or `claude-portable.exe` on Windows) in any folder
and run it. The Claude binary is saved next to the launcher so you can carry
both on a USB drive.

## Options

| Flag | Meaning |
|---|---|
| `-d`, `--dangerous` | Shorthand for `--dangerously-skip-permissions` |

All other flags are passed through to Claude unchanged.

## Examples

```bash
# Normal interactive session
./claude-portable

# Skip permission prompts (use with care)
./claude-portable --dangerous

# Pass any Claude flag through
./claude-portable --model claude-opus-4-5
```

## Downloads

Grab the binary for your platform from the
[latest release](../../releases/latest):

| Platform | File |
|---|---|
| Windows x64 | `claude-portable-windows-x64.exe` |
| Windows arm64 | `claude-portable-windows-arm64.exe` |
| Linux x64 | `claude-portable-linux-x64` |
| Linux arm64 | `claude-portable-linux-arm64` |
| macOS x64 | `claude-portable-macos-x64` |
| macOS Apple Silicon | `claude-portable-macos-arm64` |

## How it works

1. On first run: fetches the latest `@anthropic-ai/claude-code-{platform}`
   package from npm, extracts the binary next to itself.
2. On every run: execs the Claude binary directly, forwarding all args and
   signals.
3. The launcher itself is a ~3 MB statically linked Go binary with zero
   runtime dependencies.
