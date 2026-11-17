# Changelog

## v1.1.9

First published release since v1.1.5. It contains everything from the unpublished v1.1.6 to v1.1.8 builds.

- Desktop PCs without a battery skip the built-in panel check: no WMI query and no `brightness.log` entry. Laptops keep the diagnostic log.
- The installer places `SHA256SUMS.txt` (SHA-256 of `Lumistar.exe`) in the program folder, next to `Lumistar.exe`. The release file lists both downloads.
- Build scripts write `SHA256SUMS.txt` before the installer is built. A failing NSIS build stops the script.
- Installer texts show the current data folder (`%USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar`). The obsolete `settings.txt` delete is removed; uninstall removes `SHA256SUMS.txt`.
- `.gitattributes` uses `eol=lf`, so Windows Git no longer reports unchanged files as modified.
- Unused helper `pp.sh` removed; comments updated for the current data folder.

## v1.1.5

- Built-in panel check: a PC without a built-in panel (most desktops) is checked once per run. No repeated WMI queries and no repeated `brightness.log` lines.
- Removed unused code.

## v1.1.4

Hardening and compatibility release. No new features.

- Supported systems: Windows 10 and Windows 11 only. The manifest declares only this family.
- Built with the newest stable Go on the release runner (`go-version: stable`). `go.mod` language level 1.22.
- Every Windows DLL is loaded from its full path in `%SystemRoot%\System32` (blocks DLL search-order hijacking).
- A Win32 function missing on the running Windows returns 0 instead of crashing the program.
- Release files include `SHA256SUMS.txt`. The build logs the Go, NSIS and MinGW versions it used.
- Removed unused code.

## v1.1.3

- Themes: Default, Light, Black Fading to White, White Fading to Black and Black Light. Each theme has its own tray icon and its own logo colour in the panel.
- Fixed: White Fading to Black fades from white at the top to black at the bottom.
- Black Fading to White and White Fading to Black are exact opposites (purple becomes white, white becomes black).
- Default and Light keep the original purple star. The other themes use a star in their accent colour.
- The release line is slightly smaller, still bright purple, and never changes with the theme.
- Version shown under the title, in a fixed bright purple.
- Languages: 96 in six pages, English counted once. Page 2 holds the 16 most spoken Turkic languages (not Turkish or Azerbaijani). Pages 3-4 hold Persian, Kurdish, Greek and Caucasian languages. Pages 5-6 hold the remaining most spoken languages.
- Help: the microphone text describes the red button, the theme row is updated, and the language count reads 96.
- Built-in laptop brightness: diagnostics are written to `brightness.log` in the data folder.
- Data location: settings and logs moved to `%USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar`. Existing settings are copied on first start.
- Version information and manifest show 1.1.3.
- Compatibility: supported on Windows 10 and Windows 11 only. Built with the newest stable Go on the release runner.
- A missing Win32 function returns 0 instead of crashing the program.
- Security: every Windows DLL is loaded by its full path in `%SystemRoot%\System32`, so a DLL placed next to the program is never loaded.
- Removed unused code (the old header icon path).
- Release files now include `SHA256SUMS.txt`.

## v1.0.0

First public release: tray flyout, brightness (DDC/CI), volume and mute, microphone mute, Night, Stay awake, Screen off, Lock, Help, 64 languages, light and dark themes, start-up entry, installer with a choice of location.
