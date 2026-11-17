# Handoff: Lumistar

Use this file to pick up the project in any AI tool or by hand. Read it first, then `CHANGELOG.md` and `README.md`.

## Current state
- Version: **v1.1.9** (`app.rc`, `app.manifest`, `installer.nsi`, `CHANGELOG.md` all say 1.1.9).
- Tag `v1.1.4` is created by the maintainer after this code is committed. CI builds `Lumistar.exe` and `Lumistar-Setup.exe` from the tag.
- The program is Go (standard library only) plus a Windows resource file and an NSIS installer.

## Toolchain
- Go: the newest stable release. `build.yml` uses `go-version: "stable"` with `check-latest: true`, so every release build uses the newest Go. `go.mod` says `go 1.22`, the minimum language level the code needs.
- NSIS and MinGW: the newest versions Chocolatey offers on the CI runner. The author's local build used the Ubuntu packages (NSIS and MinGW from apt), which are older.
- The sandbox where this handoff was prepared had Go 1.22.2 and could not download newer toolchains, so local test builds use Go 1.22.2.

## Compatibility status
- Target: Windows 10 (22H2) and Windows 11, all supported versions. The manifest declares only this family.
- Not tested on real Windows machines by the author of this handoff. Every build so far was cross-compiled on Linux.
- Windows 7, 8, 8.1, XP and Vista: not supported.
- Wine: smoke-tested on Linux (Wine 9.0, Xvfb). Program ran for 60 seconds without crashing. `brightness.log` recorded `no instance returned for: SELECT * FROM WmiMonitorBrightness`, which is expected because Wine has no WMI brightness provider.

## Build
- Windows 7, 8, 8.1, 10, 11: declared in `app.manifest`. Built and reviewed but **not run on real Windows 7 or 8.1 hardware**.
- Windows XP and Vista: not supported. They need the audio code rewritten for the old mixer API, and there is no WMI brightness there. Needs the maintainer's approval before any work.
- Wine: smoke-tested on Linux (Wine 9.0, Xvfb). Program ran for 60 seconds without crashing. `brightness.log` recorded `no instance returned for: SELECT * FROM WmiMonitorBrightness`, which is expected because Wine has no WMI brightness provider.

## Build
- Windows: `powershell -ExecutionPolicy Bypass -File build.ps1` (needs Go, NSIS, MinGW `windres`).
- Linux cross-build: `./build.sh` (needs `golang`, `nsis`, MinGW binutils). This is how 1.1.9 was built during development.
- CI: `.github/workflows/build.yml` runs on `v*` tags and uploads the release files.

## Source map
- `main.go`: the whole UI, Win32 calls, themes, language table (`langs`, `strs`), help page, settings, release line. Text shown to users lives in `strs`, one column per language. The last column is `enus`.
- `internal_display.go`: built-in laptop brightness through WMI (`ROOT\WMI`) via COM, plus `brightLog` for diagnostics.
- `app.rc`, `app.manifest`: version resource, icons (IDs 1 default, 2 Black Fading to White, 3 White Fading to Black, 4 Black Light), manifest.
- `installer.nsi`, `README_body.txt`: installer and the README copied next to the installed program.
- `make_icons.py`, `*.raw`, `*.ico`: generated artwork (star, Claude and GitHub logos, flags, tray icons).
- `pp.sh`: preprocessor wrapper for `windres` (used by `build.sh`).
- `SECURITY.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `README.md`: documentation.

## Where user data lives
`%USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar`: `settings.txt` (theme, language), `crash.log` (unexpected errors), `brightness.log` (built-in panel failures). Settings were moved from `%LOCALAPPDATA%\Lumistar` in 1.1.3 (the data folder is unchanged since then); `migrateSettings()` copies them once.

## Open issues
1. **Laptop built-in brightness does not change on the maintainer's laptop (reported on v1.0.0).** 1.1.9 writes each WMI failure to `brightness.log` with its HRESULT. Next step: get that file from the laptop and fix the step that fails (`CoCreateInstance`, `ConnectServer`, `ExecQuery`, or `WmiSetBrightness`). Do not guess at fixes without this file.
2. **Translations.** The 96 languages are all present. The Themes label, the microphone help text and the theme help row are translated in the 64 languages from earlier versions. The 32 newest languages have partial UI words; the help page is English for most of them. Some very small Caucasian and Turkic languages are English except for their name (sah, tyv, ce, av, lez, os, kbd, ady, inh, ab, and others). Native review is needed before calling them translated.
3. **Not signed.** Windows SmartScreen may warn. Antivirus false positives are possible; report the SHA-256 of the release file to Microsoft.
4. **No MSI.** The installer is NSIS (`.exe`). An MSI would need WiX and a separate installer script; not requested.
5. **Not tested on Windows by the author of this handoff.** Every build so far was cross-compiled on Linux. Test theme switching, the tray icon swap, the release line size and the built-in brightness on a real Windows 10/11 machine.

## Languages (source bytes, GitHub's language bar)
Go about 87.6% of all files in the repository (about 92% of code only), NSIS about 4.4%, Python about 1.5% (`make_icons.py`), PowerShell about 0.4% (`build.ps1`), shell about 0.2% (`build.sh`, `pp.sh`), resource file about 0.4%, XML about 0.4%. Markdown and plain text are documentation and are not counted as code.

## Rules for changes
- Keep the code in English. Add user-visible text only to `strs`.
- Build and test on Windows 10 or 11 before tagging.
- Do not add background services, telemetry, PowerShell or new network calls.
- Bump the version in `app.rc`, `app.manifest`, `installer.nsi` and `CHANGELOG.md` together.
