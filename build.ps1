# Builds Lumistar.exe and Lumistar-Setup.exe on Windows.
# Needs: Go (https://go.dev), NSIS and MinGW (choco install nsis mingw)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not (Test-Path go.mod)) { go mod init lumistar | Out-Null }

# Icon, version information and manifest go into the executable
windres -O coff -i app.rc -o rsrc_windows_amd64.syso

# Program (no console window)
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-H windowsgui -s -w" -o Lumistar.exe .

# Installer
$makensis = (Get-Command makensis -ErrorAction SilentlyContinue).Source
if (-not $makensis) { $makensis = Join-Path ${env:ProgramFiles(x86)} "NSIS\makensis.exe" }
& $makensis -V1 installer.nsi

Write-Host "Done: Lumistar.exe and Lumistar-Setup.exe"
