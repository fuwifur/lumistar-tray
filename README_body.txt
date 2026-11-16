LUMISTAR - README
=================

Version 1.1.8

WHAT IS LUMISTAR?
-----------------
Lumistar is a small control centre for the Windows system tray. It controls the brightness of your
displays and the volume of your speakers. Click the star in the system tray to open the panel, and
click it again to close it. Press the "?" button inside the panel for a full help page.

WHERE EVERYTHING IS STORED
--------------------------
The program: the folder named "Lumistar" that contains this README.

SHA256SUMS.txt in the same folder holds the SHA-256 value of Lumistar.exe. To check the program:
  Get-FileHash .\Lumistar.exe -Algorithm SHA256
The value must match the one in SHA256SUMS.txt.

All Lumistar data (settings, theme, language, crash reports, brightness diagnostics) is kept in:

  C:\Users\<your name>\AppData\LocalLow\fuwifur\Lumistar

  settings.txt       theme and language choices
  crash.log          written only if the program hits an unexpected error
  brightness.log     written only if the built-in laptop panel cannot be controlled

To open that folder, press Win + R, type %USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar and press Enter.

Lumistar writes nothing else on your PC, except the optional "Launch at start-up" entry, which lives
under HKCU\Software\Microsoft\Windows\CurrentVersion\Run and is removed when the switch is off.

TWO KINDS OF INSTALLATION
-------------------------
Portable (Desktop, Documents, or a custom location you choose not to register)
  Nothing is registered with Windows. Removing it means deleting its program folder and the data folder above.

Standard (Program Files, AppData Local Programs, or a custom location you choose to register)
  Lumistar is listed in Settings > Apps and can be removed there. The uninstaller also removes the data folder.

HOW TO REMOVE LUMISTAR
----------------------
1. Quit Lumistar from its tray icon (right-click > Quit). Turn "Launch at start-up" off first.
2. Standard installation: Settings > Apps > Installed apps > Lumistar > Uninstall.
   Portable installation: delete the "Lumistar" program folder.
3. Delete the data folder: %USERPROFILE%\AppData\LocalLow\fuwifur\Lumistar
4. If you deleted the program folder by hand and the Start menu or Desktop shortcut is still there,
   delete the "Lumistar" shortcut (right-click > Delete).

IF YOU USED "LAUNCH AT START-UP"
--------------------------------
The standard uninstaller removes this entry. Otherwise remove it with Settings > Apps > Startup,
or run: reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v Lumistar /f

CREDITS
-------
Made by fuwifur using Claude Sonnet 5.5 and Haiku 5.5
https://guns.lol/fuwifur
