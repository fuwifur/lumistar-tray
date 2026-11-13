WHAT IS LUMISTAR?
-----------------
Lumistar is a tiny control centre for the Windows system tray. It controls the brightness of
your displays and the volume of your speakers. Click the star in the system tray to open the
panel, and click it again to close it. Press the "?" button inside the panel for a full help
page that explains every button and slider.

WHERE EVERYTHING IS STORED
--------------------------
The program: the folder named "Lumistar" that contains this README.
  Lumistar.exe   the program
  README.txt     this file
  Uninstall.exe  only present in a standard installation (see below)

Settings and data: always in your AppData Local folder, whichever location Lumistar was
installed to:
  C:\Users\<your name>\AppData\Local\Lumistar\settings.txt
This folder is created the first time you change a setting such as the theme or the language. To open it, press
Win + R, type %LOCALAPPDATA%\Lumistar and press Enter.

Lumistar stores nothing else on your PC: no ProgramData, no Temp files and no registry entries
for its own data. The one optional exception is the "Launch at start-up" switch inside the
panel: when you turn it on, Windows keeps a small start-up entry named "Lumistar" for your user
account. See "If you used Launch at start-up" at the end of this file.

If Lumistar ever stops unexpectedly it writes a short report to crash.log in that same folder.

TWO KINDS OF INSTALLATION
-------------------------
Portable (Desktop, Documents, and optionally a custom location)
  Nothing is registered with Windows. Lumistar does not appear in Settings > Apps, in the
  Control Panel, or in uninstaller tools. Removing it means deleting its program folder and
  its settings folder in AppData\Local.

Standard (Program Files, AppData Local Programs, and optionally a custom location)
  Lumistar is added to the Windows list of installed programs, gets a Start menu shortcut,
  and can be removed with Windows or with third-party uninstallers. The uninstaller also
  removes the settings folder in AppData\Local.

HOW TO REMOVE LUMISTAR - FIND YOUR LOCATION BELOW
-------------------------------------------------

Shortcuts: setup offers a Desktop shortcut and a Start menu shortcut (both ticked by default). A standard
uninstall removes them for you. For a portable copy, or after deleting the folder by hand, also delete the
"Lumistar" shortcuts from your Desktop and from the Start menu (right-click > Delete).

1) Desktop (Portable Installation)   -   folder: Desktop\Lumistar
   a. Right-click the Lumistar star in the system tray and choose Quit.
      (Optional: switch "Launch at start-up" off first.)
   b. Delete the "Lumistar" folder on your Desktop.
   c. Delete the settings folder: press Win + R, type %LOCALAPPDATA% and press Enter, then
      delete the "Lumistar" folder you see there.
   d. Empty the Recycle Bin, or press Shift + Delete to skip it.
   No uninstaller, registry entry or shortcut was created.

2) Documents (Portable Installation)   -   folder: Documents\Lumistar
   a. Right-click the Lumistar star in the system tray and choose Quit.
      (Optional: switch "Launch at start-up" off first.)
   b. Open File Explorer > Documents and delete the "Lumistar" folder.
   c. Delete the settings folder: press Win + R, type %LOCALAPPDATA% and press Enter, then
      delete the "Lumistar" folder you see there.
   d. Empty the Recycle Bin, or press Shift + Delete to skip it.
   No uninstaller, registry entry or shortcut was created.

3) Program Files   -   folder: C:\Program Files\Lumistar
   Option A - Windows Settings (recommended)
   a. Open Settings > Apps > Installed apps.
   b. Find "Lumistar", open its menu (...) and choose Uninstall.
   Option B - Control Panel
   a. Open Control Panel > Programs > Programs and Features.
   b. Select "Lumistar" and click Uninstall.
   Option C - Third-party uninstallers (for example Revo Uninstaller)
   a. Select "Lumistar", run its uninstaller, then run the advanced scan.
   Option D - Manual
   a. Quit Lumistar from the system tray, then run Uninstall.exe inside the Lumistar folder.
   All options remove the program folder, the settings folder in AppData\Local, the Start menu
   shortcut and the list entry.
   If you deleted the program folder with Shift + Delete: the list entry and the settings
   folder stay behind. Select "Lumistar" in Revo Uninstaller (or similar) and use its advanced
   scan to clean up, or do it by hand: delete %LOCALAPPDATA%\Lumistar and delete this key in the
   Registry Editor:
   HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Uninstall\Lumistar
   Also delete the Start menu shortcut "Lumistar" if it is still there.

4) AppData Local Programs   -   folder: C:\Users\<your name>\AppData\Local\Programs\Lumistar
   Use the same options A, B, C and D as for Program Files above, including the clean-up
   notes for a folder deleted with Shift + Delete.
   (AppData is a hidden folder. Press Win + R, type %LOCALAPPDATA%\Programs and press Enter
   to open the parent folder.)

5) A location you chose yourself (optional fifth choice)   -   folder: <your path>\Lumistar
   The first lines of this README tell you whether this copy is portable or standard.
   Portable copy:
   a. Quit Lumistar from the system tray.
   b. Delete the "Lumistar" folder you installed to.
   c. Delete the settings folder %LOCALAPPDATA%\Lumistar (Win + R, type the path, Enter).
   Standard copy:
   a. Use Settings > Apps > Installed apps (or Control Panel > Programs and Features, or a
      third-party uninstaller) and uninstall "Lumistar". The settings folder is removed too.
   b. If the folder was deleted by hand, clean up as described in section 3 above.

IF YOU USED "LAUNCH AT START-UP"
--------------------------------
The standard uninstaller removes this entry for you. For a portable copy, or if you deleted
the folder by hand, remove the entry in one of these ways:
  - Windows 11: Settings > Apps > Startup, switch off or remove "Lumistar".
  - Task Manager > Startup apps, right-click "Lumistar" and choose Disable.
  - Command line:
      reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v Lumistar /f

CREDITS
-------
Made by fuwifur using Claude Sonnet 5.5
Portfolio: https://guns.lol/fuwifur
GitHub:    https://github.com/fuwifur/lumistar-tray
