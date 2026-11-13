# Security

## What Lumistar does

- Shows a tray icon and a small panel window.
- Sets the brightness of external monitors through DDC/CI (the monitor's own control protocol) and of laptop built-in panels through WMI (`ROOT\WMI`), called directly over COM.
- Reads and sets the master volume and mute of the default playback device (Windows Core Audio), and mutes the default microphone when you press its button.
- Keeps the PC awake while "Stay awake" is on (`SetThreadExecutionState`) and locks it when you press Lock (`LockWorkStation`).
- When you press the sound or screenshot button, opens the Windows Sound settings (`ms-settings:sound`) or the screenshot overlay (`ms-screenclip:`). The Help page's links open in your browser.
- When "Launch at start-up" is on, adds a `Lumistar` value under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, and removes it when the switch is off.
- Stores settings in `%LOCALAPPDATA%\Lumistar\settings.txt` and writes errors to `crash.log` in the same folder.

## What Lumistar does not do

- No network connections: no telemetry, no automatic updates, no downloads.
- No keyboard hooks and no keystroke injection.
- No PowerShell, `cmd` or other hidden command-line processes.
- No services, scheduled tasks or drivers.
- No administrator rights: the program runs as the current user. The installer asks for administrator rights only when you choose Program Files.

## Reporting a vulnerability

Use GitHub's "Report a vulnerability" button (Security tab), or contact the author through https://guns.lol/fuwifur.
