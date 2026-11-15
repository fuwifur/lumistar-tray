<p align="center">
  <img src="docs/logo.png" width="128" alt="Lumistar logo">
</p>

<h1 align="center">Lumistar</h1>

<p align="center">
  <b>A mini tray control centre for Windows.</b><br>
  Brightness, volume, microphone, themes, Night, Stay awake and Screen off in one small panel.
</p>

<p align="center">
  <a href="https://github.com/fuwifur/lumistar-tray/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/fuwifur/lumistar-tray"></a>
  <a href="https://github.com/fuwifur/lumistar-tray/releases"><img alt="Downloads" src="https://img.shields.io/github/downloads/fuwifur/lumistar-tray/total"></a>
  <img alt="License" src="https://img.shields.io/badge/license-MIT-blue">
  <img alt="Platform" src="https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078D6">
  <img alt="Language" src="https://img.shields.io/badge/built%20with-Go-00ADD8">
</p>

**Current version: v1.1.5**

Lumistar is a lightweight alternative to [Twinkle Tray](https://github.com/xanderfrangos/twinkle-tray). It lives in the system tray, opens a small panel with one click, and stays out of your taskbar. It is written in Go with the standard library only, so the program is a single small `.exe`.

## Features

- **Brightness** for every display: one slider per screen and 10 to 100 preset buttons. External monitors use DDC/CI. Laptop built-in panels use WMI.
- **Volume** with a master slider, 10 to 100 presets, and mute.
- **Microphone mute**: the button turns red while the microphone is muted.
- **Themes**: Default, Light, Black Fading to White, White Fading to Black and Black Light. Each theme has its own tray icon, and the panel logo matches the tray icon.
- **Night**: dims every display to 20% in one click and restores your levels.
- **Stay awake**, **Screen off**, **Lock**, **Launch at start-up**.
- **Quick links**: Sound settings (gear) and the Windows screenshot tool.
- **96 languages** in six pages, English included. The panel and the help page are translated into most of them.
- **Help page** that explains every control.

## Supported systems

- Windows 10 (version 22H2) and Windows 11, all versions that Microsoft still supports.
- Older Windows versions and Wine are not supported.

## Installation

Download `Lumistar-Setup.exe` from the [Releases](https://github.com/fuwifur/lumistar-tray/releases/latest) page and run it. Or download `Lumistar.exe` for a copy that needs no installation.

The installer asks where to put Lumistar: Desktop (portable, default), Documents (portable), Program Files, AppData Local Programs (`%LOCALAPPDATA%\Programs\Lumistar`), or a folder of your choice. Standard installations appear in Settings > Apps and can be removed there.

## Where your data lives

All Lumistar data (settings, theme, language, crash and brightness logs) is kept in one folder:

```
C:\Users\<your name>\AppData\LocalLow\fuwifur\Lumistar
```

## Build from source (Windows 10 or 11)

1. Install [Go](https://go.dev), then run `choco install nsis mingw -y`. NSIS builds the installer; MinGW's `windres` embeds the icons, version information and manifest.
2. In the project folder run `powershell -ExecutionPolicy Bypass -File build.ps1`.

The release build uses the newest stable Go (`go version` should say the current release). You get `Lumistar.exe` and `Lumistar-Setup.exe`. Every push of a `v*` tag runs the same build on GitHub Actions and attaches both files and `SHA256SUMS.txt` to the release. Check a download with `Get-FileHash Lumistar-Setup.exe -Algorithm SHA256`.

## Known limitations

- External monitors need **DDC/CI** switched on in their own on-screen menu. Some monitors and docks do not support it.
- Laptop built-in panels use WMI. If the built-in slider does nothing on your laptop, send `brightness.log` from the data folder above.
- Windows does not offer per-app volume here; Lumistar changes the master volume of the default playback device.
- Translations of the newest languages are partly in English on the help page.

## Antivirus and SmartScreen

Lumistar is not code-signed yet, so Windows SmartScreen may warn before the first run ("Windows protected your PC": More info > Run anyway). Lumistar does not inject keystrokes, hide command windows, run PowerShell or open network connections. [SECURITY.md](SECURITY.md) lists exactly what it does and does not do.

## Credits

- Made by [fuwifur](https://guns.lol/fuwifur) using Claude Sonnet 5.5 and Haiku 5.5. [Claude](https://claude.ai) is a trademark of Anthropic and is used here only as a link.
- Inspired by [Twinkle Tray](https://github.com/xanderfrangos/twinkle-tray) (MIT). Lumistar is not affiliated with it.
- Country flags from [flag-icons](https://github.com/lipis/flag-icons) (MIT).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © 2026 fuwifur
