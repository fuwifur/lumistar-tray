; Lumistar installer. The program goes into a folder named "Lumistar"; its settings always live in %LOCALAPPDATA%\Lumistar.
Unicode true
ManifestDPIAware true
Name "Lumistar"
OutFile "Lumistar-Setup.exe"
InstallDir "$DESKTOP\Lumistar"
RequestExecutionLevel user
SetCompressor /SOLID lzma
ShowInstDetails nevershow

!include "MUI2.nsh"
!include "nsDialogs.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!insertmacro GetParameters
!insertmacro GetOptions

!define APPNAME "Lumistar"
!define VERSION "0.1.0"
!define UNKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Lumistar"
!define RUNKEY "Software\Microsoft\Windows\CurrentVersion\Run"

!define MUI_ICON "setup.ico"
!define MUI_UNICON "setup.ico"
!define MUI_FINISHPAGE_RUN "$INSTDIR\Lumistar.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch Lumistar"
!define MUI_FINISHPAGE_SHOWREADME "$INSTDIR\README.txt"
!define MUI_FINISHPAGE_SHOWREADME_TEXT "Open README.txt"
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED

Var Choice        ; 1 Desktop, 2 Documents, 3 Program Files, 4 AppData Local Programs, 5 custom
Var CustomDir
Var Registered    ; 1 = standard installation, 0 = portable
Var Radio1
Var Radio2
Var Radio3
Var Radio4
Var PathLabel

Page custom LocationPage LocationLeave
!define MUI_COMPONENTSPAGE_TEXT_TOP "Choose which shortcuts Lumistar should create. Both are ticked by default; untick any you do not want."
!define MUI_COMPONENTSPAGE_TEXT_COMPLIST "Shortcuts:"
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Function .onInit
  StrCpy $Choice 1
  StrCpy $CustomDir ""
  ${GetParameters} $0
  ClearErrors
  ${GetOptions} $0 "/PROGRAMFILES" $1
  ${IfNot} ${Errors}
    StrCpy $Choice 3
  ${EndIf}
FunctionEnd

; Works out $INSTDIR from the current choice. The folder is always named "Lumistar".
Function ResolveDir
  ${If} $Choice == 1
    StrCpy $INSTDIR "$DESKTOP\Lumistar"
  ${ElseIf} $Choice == 2
    StrCpy $INSTDIR "$DOCUMENTS\Lumistar"
  ${ElseIf} $Choice == 3
    StrCpy $INSTDIR "$PROGRAMFILES64\Lumistar"
  ${ElseIf} $Choice == 4
    StrCpy $INSTDIR "$LOCALAPPDATA\Programs\Lumistar"
  ${Else}
    StrCpy $INSTDIR "$CustomDir"
    StrCpy $0 "$INSTDIR" 1 -1
    ${If} $0 == "\"
      StrCpy $INSTDIR "$INSTDIR" -1
    ${EndIf}
    StrCpy $0 "$INSTDIR" 9 -9
    ${If} $0 != "\Lumistar"
      StrCpy $INSTDIR "$INSTDIR\Lumistar"
    ${EndIf}
  ${EndIf}
FunctionEnd

Function UpdateLabel
  Call ResolveDir
  ${NSD_SetText} $PathLabel "Lumistar will be installed to:$\r$\n$INSTDIR"
FunctionEnd

Function OnRadio
  Pop $0
  ${If} $0 == $Radio1
    StrCpy $Choice 1
  ${ElseIf} $0 == $Radio2
    StrCpy $Choice 2
  ${ElseIf} $0 == $Radio3
    StrCpy $Choice 3
  ${Else}
    StrCpy $Choice 4
  ${EndIf}
  Call UpdateLabel
FunctionEnd

Function OnCustom
  Pop $0
  nsDialogs::SelectFolderDialog "Choose the folder that should contain Lumistar" "$DOCUMENTS"
  Pop $0
  ${If} $0 != error
    StrCpy $CustomDir $0
    StrCpy $Choice 5
    ${NSD_Uncheck} $Radio1
    ${NSD_Uncheck} $Radio2
    ${NSD_Uncheck} $Radio3
    ${NSD_Uncheck} $Radio4
    Call UpdateLabel
  ${EndIf}
FunctionEnd

Function LocationPage
  !insertmacro MUI_HEADER_TEXT "Choose install location" "The program goes into the Lumistar folder below. Its settings are always kept in $LOCALAPPDATA\Lumistar."
  nsDialogs::Create 1018
  Pop $0

  ${NSD_CreateRadioButton} 0 0 100% 11u "Desktop  (Portable Installation)"
  Pop $Radio1
  ${NSD_AddStyle} $Radio1 ${WS_GROUP}
  ${NSD_OnClick} $Radio1 OnRadio
  ${NSD_CreateLabel} 12u 11u 100% 9u "$DESKTOP\Lumistar"
  Pop $0

  ${NSD_CreateRadioButton} 0 24u 100% 11u "Documents  (Portable Installation)"
  Pop $Radio2
  ${NSD_OnClick} $Radio2 OnRadio
  ${NSD_CreateLabel} 12u 35u 100% 9u "$DOCUMENTS\Lumistar"
  Pop $0

  ${NSD_CreateRadioButton} 0 48u 100% 11u "Program Files"
  Pop $Radio3
  ${NSD_OnClick} $Radio3 OnRadio
  ${NSD_CreateLabel} 12u 59u 100% 9u "$PROGRAMFILES64\Lumistar"
  Pop $0

  ${NSD_CreateRadioButton} 0 72u 100% 11u "AppData Local Programs"
  Pop $Radio4
  ${NSD_OnClick} $Radio4 OnRadio
  ${NSD_CreateLabel} 12u 83u 100% 9u "$LOCALAPPDATA\Programs\Lumistar"
  Pop $0

  ${NSD_CreateButton} 0 98u 132u 14u "Choose another location..."
  Pop $0
  ${NSD_OnClick} $0 OnCustom

  ${NSD_CreateLabel} 0 118u 100% 24u ""
  Pop $PathLabel

  ${If} $Choice == 3
    ${NSD_Check} $Radio3
  ${Else}
    ${NSD_Check} $Radio1
  ${EndIf}
  Call UpdateLabel
  nsDialogs::Show
FunctionEnd

; Sets $0 to 1 when the folder in $INSTDIR can be written to, otherwise 0.
Function CheckWritable
  StrCpy $2 0
  ${If} ${FileExists} "$INSTDIR\*.*"
    StrCpy $2 1
  ${EndIf}
  ClearErrors
  CreateDirectory "$INSTDIR"
  FileOpen $1 "$INSTDIR\.write-test" w
  ${If} ${Errors}
    StrCpy $0 0
  ${Else}
    FileClose $1
    Delete "$INSTDIR\.write-test"
    StrCpy $0 1
  ${EndIf}
  ${If} $2 == 0
    RMDir "$INSTDIR"
  ${EndIf}
FunctionEnd

Function LocationLeave
  Call ResolveDir
  Call CheckWritable
  ${If} $0 == 0
    ${If} $Choice == 3
      MessageBox MB_YESNO|MB_ICONQUESTION "Installing to Program Files needs administrator permission.$\r$\n$\r$\nRestart the installer as administrator?" IDYES elevate
      Abort
      elevate:
      ExecShell "runas" "$EXEPATH" "/PROGRAMFILES"
      Quit
    ${Else}
      MessageBox MB_OK|MB_ICONEXCLAMATION "Lumistar cannot write to this location.$\r$\n$\r$\nPlease choose another one."
      Abort
    ${EndIf}
  ${EndIf}

  StrCpy $Registered 0
  ${If} $Choice == 3
  ${OrIf} $Choice == 4
    StrCpy $Registered 1
  ${ElseIf} $Choice == 5
    MessageBox MB_YESNO|MB_ICONQUESTION "Add Lumistar to the Windows list of installed programs?$\r$\n$\r$\nYES: it can be removed from Settings > Apps and by uninstaller tools.$\r$\nNO: portable copy that is not listed anywhere; remove it by deleting its folder and the Lumistar folder in AppData\Local." IDNO custom_portable
    StrCpy $Registered 1
    custom_portable:
  ${EndIf}
FunctionEnd

Section "Lumistar (required)" SecMain
  SectionIn RO
  SetShellVarContext current
  nsExec::Exec 'taskkill /IM Lumistar.exe /F'
  Sleep 400
  SetOutPath "$INSTDIR"
  File "Lumistar.exe"

  ; README.txt: a short header about this copy, then the shared removal guide.
  InitPluginsDir
  File "/oname=$PLUGINSDIR\readme_body.txt" "README_body.txt"
  FileOpen $1 "$INSTDIR\README.txt" w
  FileWrite $1 "LUMISTAR - README$\r$\n$\r$\n"
  ${If} $Registered == 1
    FileWrite $1 "THIS COPY: standard installation (listed in Settings > Apps and in the Control Panel).$\r$\n"
  ${Else}
    FileWrite $1 "THIS COPY: portable installation (not listed in Settings, the Control Panel or uninstaller tools).$\r$\n"
  ${EndIf}
  FileWrite $1 "LOCATION: $INSTDIR$\r$\n$\r$\n"
  FileOpen $2 "$PLUGINSDIR\readme_body.txt" r
  readme_loop:
    FileRead $2 $3
    ${If} ${Errors}
      Goto readme_done
    ${EndIf}
    FileWrite $1 $3
    Goto readme_loop
  readme_done:
  FileClose $2
  FileClose $1

  ; Program Files is read-only for normal users: let the installing user keep settings there.
  ${If} $Choice == 3
    ReadEnvStr $0 USERNAME
    nsExec::Exec 'icacls "$INSTDIR" /grant "$0:(OI)(CI)M" /T /C /Q'
  ${EndIf}

  ${If} $Registered == 1
    WriteUninstaller "$INSTDIR\Uninstall.exe"
    WriteRegStr HKCU "${UNKEY}" "DisplayName" "Lumistar"
    WriteRegStr HKCU "${UNKEY}" "DisplayVersion" "${VERSION}"
    WriteRegStr HKCU "${UNKEY}" "Publisher" "fuwifur"
    WriteRegStr HKCU "${UNKEY}" "DisplayIcon" "$INSTDIR\Lumistar.exe"
    WriteRegStr HKCU "${UNKEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKCU "${UNKEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
    WriteRegDWORD HKCU "${UNKEY}" "NoModify" 1
    WriteRegDWORD HKCU "${UNKEY}" "NoRepair" 1
  ${EndIf}
SectionEnd

Section "Create desktop shortcut" SecDesktop
  SetShellVarContext current
  CreateShortcut "$DESKTOP\Lumistar.lnk" "$INSTDIR\Lumistar.exe"
SectionEnd

Section "Create Start menu shortcut" SecStart
  SetShellVarContext current
  CreateShortcut "$SMPROGRAMS\Lumistar.lnk" "$INSTDIR\Lumistar.exe"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain} "The Lumistar program and its README.txt (always installed)."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Adds a Lumistar shortcut to your Desktop."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecStart} "Adds Lumistar to the Start menu."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  SetShellVarContext current
  nsExec::Exec 'taskkill /IM Lumistar.exe /F'
  Sleep 400
  Delete "$SMPROGRAMS\Lumistar.lnk"
  Delete "$DESKTOP\Lumistar.lnk"
  DeleteRegKey HKCU "${UNKEY}"
  DeleteRegValue HKCU "${RUNKEY}" "Lumistar"
  RMDir /r "$LOCALAPPDATA\Lumistar"
  ; Only ever remove a folder that is really named "Lumistar".
  StrCpy $0 "$INSTDIR" 9 -9
  ${If} $0 == "\Lumistar"
    RMDir /r "$INSTDIR"
  ${Else}
    Delete "$INSTDIR\Lumistar.exe"
    Delete "$INSTDIR\settings.txt"
    Delete "$INSTDIR\README.txt"
    Delete "$INSTDIR\Uninstall.exe"
    RMDir "$INSTDIR"
  ${EndIf}
SectionEnd
