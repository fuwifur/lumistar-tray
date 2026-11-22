LUMISTAR - README
=================

Version 1.4.5

WHAT IS LUMISTAR?
-----------------
Lumistar is a small control center for the Windows system tray. It controls the brightness of your
displays, the volume of your speakers and the microphone mute. Click the star in the system tray to
open the panel, and click it again to close it. Press the "?" button inside the panel for a full help page.

KEYBOARD AND HOTKEYS
--------------------
In the panel: Tab and Shift+Tab (or Up and Down) move between controls, Left and Right change a slider,
and Enter or Space presses a button. Esc closes the panel.

Global hotkeys, active while Lumistar runs: Ctrl+Alt+Up and Ctrl+Alt+Down change the brightness by 10,
and Ctrl+Alt+M mutes or unmutes the speakers.

WHERE EVERYTHING IS STORED
--------------------------
The program: the folder named "Lumistar" that contains this README.

SHA256SUMS.txt in the same folder holds the SHA-256 value of Lumistar.exe. To check the program, run:
  Get-FileHash .\Lumistar.exe -Algorithm SHA256
The value must match the one in SHA256SUMS.txt.

Lumistar keeps all its data in this folder:

  C:\Users\<your name>\AppData\LocalLow\fuwifur\Lumistar

  settings.txt       theme and language
  SHA256SUMS.txt     SHA-256 value of the Lumistar.exe that last ran
  README.txt         a description of the files in this folder
  version.txt        the Lumistar version and its build
  install-path.txt   the folder of the Lumistar.exe that last ran
  last-start.txt     time, laptop or desktop, theme and language of the last start
  lumistar.log       one line for each start
  brightness.log     written only on a laptop whose built-in panel cannot be controlled
  crash.log          written only after an unexpected error

To open that folder, press Win + R, type %USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar and press Enter.

Lumistar writes only these files, plus the optional "Launch at startup" entry. That entry lives under
HKCU\Software\Microsoft\Windows\CurrentVersion\Run and is removed when the switch is off.

TWO KINDS OF INSTALLATION
-------------------------
Portable (Desktop, Documents, or a custom location you choose not to register)
  Nothing is registered with Windows. To remove it, delete its program folder and the data folder above.

Standard (Program Files, AppData Local Programs, or a custom location you choose to register)
  Lumistar is listed in Settings > Apps and can be removed there. The uninstaller keeps the data folder.
  Delete it yourself if you want all the data removed.

HOW TO REMOVE LUMISTAR
----------------------
1. Turn "Launch at startup" off, then quit Lumistar from its tray icon (right-click > Quit).
2. Standard installation: Settings > Apps > Installed apps > Lumistar > Uninstall.
   Portable installation: delete the "Lumistar" program folder.
3. Delete the data folder: %USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar
4. If you deleted the program folder by hand and a Start menu or Desktop shortcut remains, delete the
   "Lumistar" shortcut.

IF YOU USED "LAUNCH AT STARTUP"
-------------------------------
The standard uninstaller removes this entry. Otherwise remove it in Settings > Apps > Startup, or run:
  reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v Lumistar /f

CREDITS
-------
Made by fuwifur using Claude Sonnet 5.5 and Haiku 5.5
https://guns.lol/fuwifur
