# Handoff: Lumistar

Read this file first, then `CHANGELOG.md` and `README.md`.

## Current state
- Version: **v1.4.2** (`main.go`, `app.rc`, `app.manifest`, `installer.nsi`, `README.md`, `README_body.txt`, `CHANGELOG.md` must all say the same version).
- Tags (`v1.x.y`) are created by the maintainer after the code is committed. CI builds `Lumistar.exe` and `Lumistar-Setup.exe` from the tag and attaches them with `SHA256SUMS.txt`.
- The program is Go (standard library only), a Windows resource file and an NSIS installer.

## Toolchain
- Go: `build.yml` uses `go-version: "stable"` with `check-latest: true`, so release builds use the newest stable Go. `go.mod` says `go 1.22`, the minimum language level the code needs.
- NSIS and MinGW: the versions Chocolatey provides on the CI runner.
- The local development sandbox had Go 1.22.2, NSIS 3.09 and Wine 9.0. Local results can differ from CI. The installer is built as a 32-bit NSIS stub (the GitHub runner's NSIS has no 64-bit stub). The program itself is 64-bit.

## Compatibility
- Target: Windows 10 (22H2) and Windows 11. The manifest declares only this family.
- Windows 7, 8, 8.1, XP and Vista: not supported. XP and Vista would need the audio code rewritten for the old mixer API and have no WMI brightness; that needs the maintainer's approval first.
- Tested by the maintainer on Windows: installing and running the release works, and the panel works.
- Wine (9.0, Xvfb) was used for smoke tests on Linux only. Wine has no WMI brightness provider, so the built-in brightness does nothing there.

## Build
- Windows: `powershell -ExecutionPolicy Bypass -File build.ps1` (needs Go, NSIS, MinGW `windres`).
- Linux cross-build: `./build.sh` (needs `golang`, `nsis`, MinGW binutils). `build.sh` uses `pp.sh` as the `windres` preprocessor.
- CI: `.github/workflows/build.yml` runs on `v*` tags, builds with `build.ps1`, writes `SHA256SUMS.txt`, and publishes the release.

## Source map
- `main.go`: the whole UI, Win32 calls, themes, the language table (`langs`, `strs`), the help page, settings, the release line. Text shown to users lives in `strs`, one column per language; the last column is `enus`. Missing entries fall back to Russian for the languages in `russianFallback`, otherwise to English.
- `internal_display.go`: built-in brightness through WMI (`ROOT\WMI`) via COM, plus `brightLog` for diagnostics. A failure is written to `brightness.log` only on a PC that has a battery.
- `keyboard.go`: keyboard navigation, global hotkeys (`RegisterHotKey`), the display selection and the brightness steps.
- `mic.go`: the microphone lock (button, saved level, 1.5 s guard).
- `datafiles.go`: the files written to the data folder on every start (see below).
- `app.rc`, `app.manifest`: version resource, icons (IDs 1 default, 2 Black Fading to White, 3 White Fading to Black, 4 Black Light), manifest.
- `installer.nsi`, `README_body.txt`: the installer and the README copied next to the installed program.
- `make_icons.py`, `*.raw`, `*.ico`: generated artwork (star, Claude and GitHub logos, flags, tray icons).
- `pp.sh`: preprocessor wrapper for `windres`, used by `build.sh`.
- `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `README.md`, `ACCESSIBILITY.md`, `CODE_OF_CONDUCT.md`, `.github/`: documentation and GitHub templates.

## Where user data lives
`%USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar`. Every start writes `settings.txt` (created when missing, updated on theme or language change), `SHA256SUMS.txt` (SHA-256 of the running `Lumistar.exe`), `README.txt`, `version.txt`, `install-path.txt`, `last-start.txt` and one line in `lumistar.log`. `brightness.log` (laptops only, when the built-in panel does not answer) and `crash.log` (after an unexpected error) appear only when needed.
- Settings were moved from `%LOCALAPPDATA%\Lumistar` in 1.1.3; `migrateSettings()` copies them once.
- The uninstaller does not delete this folder.

## Open issues
1. **Built-in laptop brightness.** It was reported not to change on at least one laptop (on v1.0.0, and on a test build tried on a friend's laptop). Next step: get `brightness.log` from that laptop and fix the step that fails (`CoCreateInstance`, `ConnectServer`, `ExecQuery` or `WmiSetBrightness`). Do not guess at fixes without this file. On a desktop there is no built-in panel and no `brightness.log` entry is written.
2. **Translations.** All 96 languages exist, but many entries are empty. The 32 newest languages have partial UI words and mostly English help text. The minority languages of Russia and the Caucasus (ab, av, ce, lez, os, kbd, ady, inh, tt, ba, cv, sah, tyv, krc, kum) show Russian where their own text is missing. Native review is still needed before calling any language fully translated.
3. **Not signed.** Windows SmartScreen may warn. Antivirus false positives are possible; report the SHA-256 of the release file to Microsoft.
4. **No MSI.** The installer is NSIS (`.exe`). An MSI would need WiX and a separate installer script.
5. **Still to check on a real Windows machine:** the built-in brightness on a laptop, the tray icon swap between themes, and the release line size.

## Languages (source bytes, GitHub's language bar)
Go about 87.6% of all files in the repository (about 92% of code only), NSIS about 4.4%, Python about 1.5% (`make_icons.py`), PowerShell about 0.4% (`build.ps1`), shell about 0.2% (`build.sh`, `pp.sh`), resource file about 0.4%, XML about 0.4%. Markdown and plain text are documentation and are not counted as code.

## Rules for changes
- Keep the code in English. Add user-visible text only to `strs`, and never invent a translation.
- Build and test on Windows 10 or 11 before tagging.
- Do not add background services, telemetry, PowerShell or new network calls.
- Bump the version in every file listed under "Current state" together.
