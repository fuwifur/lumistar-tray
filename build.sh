#!/bin/sh
# Builds Lumistar.exe and Lumistar-Setup.exe from Linux/macOS (needs: go, nsis, mingw-w64 binutils)
set -e
[ -f go.mod ] || go mod init lumistar
x86_64-w64-mingw32-windres --preprocessor=./pp.sh -O coff -i app.rc -o rsrc_windows_amd64.syso
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H windowsgui -s -w" -o Lumistar.exe .
makensis -V1 installer.nsi
