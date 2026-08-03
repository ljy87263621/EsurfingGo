# EsurfingGo Maintenance Notes

## Decision

I am taking over `xxmod/EsurfingGo` locally because it has the best maintenance base among the evaluated ESurfing clients:

- Recent upstream activity and a small Go codebase.
- MIT license and single-binary distribution model.
- Existing unit tests for session, cipher, redirect handling, state, and utilities.
- Cross-platform CI/release workflow already targets Windows, Linux, macOS, ARM, and MIPS.
- The primary local goal is replacing the official Windows ESurfing client on this machine, so local config, logs, interface binding, and autostart are first-class maintenance concerns.

## Local Setup

The local takeover branch is `codex/takeover-esurfinggo`. The upstream repository is kept as the `upstream` remote; the user's repository is configured as `origin` when publishing this branch.

Go is available locally at:

```powershell
D:\Develope\Go\1.25.7\go\bin\go.exe
```

## Verification Commands

Run these before publishing changes:

```powershell
& 'D:\Develope\Go\1.25.7\go\bin\gofmt.exe' -w *.go network\*.go cipher\*.go utils\*.go model\*.go
& 'D:\Develope\Go\1.25.7\go\bin\go.exe' test ./... -v -count=1
& 'D:\Develope\Go\1.25.7\go\bin\go.exe' vet ./...
& 'D:\Develope\Go\1.25.7\go\bin\go.exe' build -trimpath -ldflags='-s -w' -o bin\esurfing-windows-amd64.exe .
```

## Immediate Policy

- Do not log plaintext credentials, SMS codes, tickets, encrypted payload previews, raw response bodies, or URL query strings.
- Preserve logs that help diagnose connectivity, but prefer lengths and sanitized URLs over raw protocol content.
- Keep release artifacts dependency-free for Windows users unless a platform constraint makes that impossible.
- Do not commit `esurfing.local.json`, log files, or any real campus-network credentials.
- The Windows release is a Portable package containing the native GUI executable, README, license, config template, and Windows operating notes.
- The GUI supports tray minimization, local plaintext credential storage, current-user autostart, and Clash/Mihomo TUN-safe portal detection.

## Release v1.2.0

- Release name: `EsurfingGo v1.2.0 - Windows Native GUI`
- Local asset: `dist/EsurfingGo-v1.2.0-windows-amd64.zip`
- SHA-256: `d95f90777813a0aebd9438f16149dfa958f64c855d53b24bfa8f96acc7ff00da`
- GitHub upload is pending until the local GitHub network path and token authentication are restored.

## Next Backlog

- Add an opt-in debug mode with explicit unsafe-log warnings if raw protocol traces are ever needed.
- Normalize release naming between `esurfinggo` usage text and produced `esurfing-*` binaries.
- Consider extending the Windows GUI only after the current native tray and TUN-safe flow remains stable on additional campus networks.
- Expand tests around portal config parsing and SMS verification request construction.
- Decide whether to redact generated MAC/client identifiers from normal logs for stricter privacy defaults.
