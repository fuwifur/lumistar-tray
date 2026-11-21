# Security

## What Lumistar does
- Shows a tray icon and a small panel window.
- Sets display brightness: DDC/CI for external monitors, and WMI (`ROOT\WMI`), called directly over COM, for laptop built-in panels.
- Reads and sets the master volume and mute of the default playback device (Windows Core Audio), and the microphone mute.
- Keeps the PC awake while "Stay awake" is on (`SetThreadExecutionState`) and locks it when you press Lock (`LockWorkStation`).
- Opens Windows pages or tools only when you press their button: Sound settings (`ms-settings:sound`) and the screenshot overlay (`ms-screenclip:`). The Help page's links open in your browser.
- Optionally writes a `Lumistar` value under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` when "Launch at start-up" is on, and removes it when it is off.
- Stores its data (settings, logs) only in `%USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar`.
- Reads its own `Lumistar.exe` once per start to compute its SHA-256 value, which it writes to `SHA256SUMS.txt` in the data folder. The value is not sent anywhere.

## Hardening in place
- Every Windows DLL is loaded from its full path in `%SystemRoot%\System32`. A DLL placed in the program's folder is never loaded (DLL search-order hijacking).
- Every Win32 call goes through one helper that checks the function exists first. A missing function returns 0 instead of crashing.
- Release files are published with `SHA256SUMS.txt` so a download can be checked.

## What Lumistar does not do
- No network connections, no telemetry, no automatic updates, no downloads.
- No keyboard hooks and no keystroke injection.
- No PowerShell, `cmd` or other hidden command-line processes.
- No services, scheduled tasks or drivers.
- No administrator rights: the program runs as the current user. The installer asks for administrator rights only when you choose Program Files.

## Reporting a vulnerability
Use GitHub's "Report a vulnerability" button (Security tab), or contact the author through https://guns.lol/fuwifur.
