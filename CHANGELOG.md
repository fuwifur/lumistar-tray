# Changelog

## Unreleased

- Maintenance: all GitHub Actions moved to current major versions that run on Node 24 (checkout v7, setup-go v7, upload-artifact v7, action-gh-release v3, CodeQL v4).
- Build: `go.sum` added so the Go module cache works (the project has no external modules).
- CodeQL: explicit workflow (`codeql.yml`) that analyses Go on a Windows runner, because the Go sources are Windows-only.
- Dependabot keeps the GitHub Actions versions up to date weekly.

## v1.0.0

First public release.

- Tray icon with a flyout panel: click to open, click again to close, Esc or a click elsewhere to dismiss.
- Brightness for external monitors (DDC/CI) and laptop built-in panels (WMI), with per-display sliders and 10 to 100 preset buttons.
- Master volume with 10 to 100 presets, mute, and a microphone mute that turns red.
- Night (dims all displays to 20% and back), Stay awake, Screen off, Lock and a Help page.
- Quick links: Sound settings and the screenshot overlay.
- 64 languages in four pages, plus EN US / EN UK.
- Light and dark themes, launch at start-up.
- Installer with Desktop, Documents, Program Files, AppData Local Programs or a custom location; portable and standard installations.
- Settings in `%LOCALAPPDATA%\Lumistar\settings.txt`, crash reports in `crash.log` in the same folder.
- Version information and an application manifest are embedded in the executable (`asInvoker`, no administrator rights needed).
- No keystroke injection, no hidden PowerShell and no `reg.exe`: the start-up entry is written through the Windows API.
