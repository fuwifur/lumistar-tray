<p align="center">
  <img src="docs/logo.png" width="128" alt="Lumistar logo">
</p>

<h1 align="center">Lumistar</h1>

<p align="center">
  <b>The ultimate mini tray control centre for Windows.</b><br>
  Brightness, volume, microphone, night mode, stay-awake and screen-off in one small panel.
</p>

<p align="center">
  <a href="https://github.com/fuwifur/lumistar-tray/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/fuwifur/lumistar-tray"></a>
  <a href="https://github.com/fuwifur/lumistar-tray/releases"><img alt="Downloads" src="https://img.shields.io/github/downloads/fuwifur/lumistar-tray/total"></a>
  <img alt="License" src="https://img.shields.io/badge/license-MIT-blue">
  <img alt="Platform" src="https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078D6">
  <img alt="Language" src="https://img.shields.io/badge/built%20with-Go-00ADD8">
</p>

Lumistar is a lightweight alternative to [Twinkle Tray](https://github.com/xanderfrangos/twinkle-tray). It lives in the system tray, opens a small panel with one click and stays out of your taskbar. It is written in Go with the standard library only, so the program is a single small `.exe` that starts instantly.

## Features

- **Brightness** for every display: a slider per screen and 10 to 100 preset buttons. External monitors use DDC/CI, laptop built-in panels use WMI.
- **Volume** with a master slider, 10 to 100 presets and a mute button.
- **Microphone mute**: the button turns red with a line through it while the microphone is muted.
- **Night**: dims every display to 20% in one click and restores your previous levels.
- **Stay awake**: keeps the PC and screens from sleeping.
- **Screen off**, **Lock**, and **Launch at start-up**.
- **Quick links**: Sound settings (gear) and the Windows screenshot tool.
- **Help page** inside the panel that explains every button.
- **64 languages** in four pages, including EN US and EN UK. The panel and the help page are translated into most of them.
- **Light and dark themes**. Your choice is remembered.

## Installation

Download the latest `Lumistar-Setup.exe` from the [Releases](https://github.com/fuwifur/lumistar-tray/releases/latest) page and run it.

The installer asks where to put Lumistar:

| Choice | Folder | Type |
|---|---|---|
| Desktop (default) | `Desktop\Lumistar` | Portable, not listed in Windows |
| Documents | `Documents\Lumistar` | Portable, not listed in Windows |
| Program Files | `C:\Program Files\Lumistar` | Standard, listed in Settings > Apps |
| AppData Local Programs | `%LOCALAPPDATA%\Programs\Lumistar` | Standard, listed in Settings > Apps |
| Choose another location | any folder | Asks whether to list it in Windows |

Every installation writes a `README.txt` next to the program with removal steps for its location. A standalone `Lumistar.exe` is also published with each release; it needs no installation.

Settings are always stored in `%LOCALAPPDATA%\Lumistar\settings.txt`, whichever folder you install to.

## Usage

- **Left-click** the star in the tray to open the panel; click it again, press **Esc**, or click anywhere else to close it.
- **Right-click** the tray icon for **Open Lumistar** and **Quit**.
- Press **?** in the panel for the full help page, which describes every control in your language.

## Build from source

Windows (10 or 11):

1. Install [Go](https://go.dev), then run `choco install nsis mingw -y`. NSIS builds the installer; MinGW's `windres` embeds the icon, version information and manifest.
2. In the project folder run `powershell -ExecutionPolicy Bypass -File build.ps1`

You get `Lumistar.exe` (the program) and `Lumistar-Setup.exe` (the installer).

Linux (cross-compiling for Windows, used by the author): see `build.sh`. It needs `golang`, `nsis` and the MinGW `binutils`.

Every push of a `v*` tag runs the same build on GitHub Actions and attaches both files to the release.

## Known limitations

- External monitors need **DDC/CI** switched on in their own on-screen menu. Some monitors and docks do not support it.
- Built-in laptop panels are read and set through WMI, so they work on most laptops only.
- Windows does not offer per-app volume here; Lumistar changes the master volume of the default playback device.
- Translations are community-quality and written without native review. Some languages still show English on the help page.

## Antivirus and SmartScreen

Lumistar is not code-signed yet, so Windows SmartScreen may warn before the first run ("Windows protected your PC": More info > Run anyway). Antivirus engines sometimes flag new unsigned programs by mistake. Lumistar does not inject keystrokes, hide command windows, run PowerShell or open network connections. [SECURITY.md](SECURITY.md) lists exactly what it does and does not do.

If your antivirus flags a release, please report it to the vendor and to Microsoft (Microsoft Security Intelligence, file submission) and mention the file's SHA-256 from the release page.

## Credits

- Made by [fuwifur](https://guns.lol/fuwifur) using Claude Sonnet 5.5 and Haiku 5.5. [Claude](https://claude.ai) is a trademark of Anthropic and is used here only as a link.
- Inspired by [Twinkle Tray](https://github.com/xanderfrangos/twinkle-tray) by Xander Frangos (MIT). Lumistar is not affiliated with it.
- Country flags from [flag-icons](https://github.com/lipis/flag-icons) (MIT).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © 2026 fuwifur
