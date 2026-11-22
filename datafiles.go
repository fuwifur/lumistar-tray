package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ensureDataFiles writes the same basic files to the data folder every time Lumistar starts:
// settings.txt (only when it is missing), SHA256SUMS.txt for the running program, README.txt and lumistar.log.
func ensureDataFiles() {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	if _, err := os.Stat(settingsPath()); err != nil {
		saveTheme(false)
	}
	writeDataFile("SHA256SUMS.txt", programChecksum())
	writeDataFile("README.txt", strings.ReplaceAll(dataReadme, "\n", "\r\n"))
	writeDataFile("version.txt", versionInfo())
	writeDataFile("install-path.txt", installPath())
	writeDataFile("last-start.txt", lastStart())
	startupLog()
}

// writeDataFile replaces one file in the data folder. An empty content leaves the file untouched.
func writeDataFile(name, content string) {
	if content == "" {
		return
	}
	os.WriteFile(filepath.Join(dataDir(), name), []byte(content), 0o644)
}

// versionInfo describes the build: the release, the Go version and the target.
func versionInfo() string {
	return "Lumistar " + releaseText + "\r\n" +
		"Built with " + runtime.Version() + " for " + runtime.GOOS + "/" + runtime.GOARCH + "\r\n"
}

// installPath says which Lumistar.exe is running, so the install location is known.
func installPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return "Running from: " + exe + "\r\n"
}

// lastStart holds the details of the most recent start only; lumistar.log keeps the history.
func lastStart() string {
	return "Started: " + time.Now().Format("2006-01-02 15:04:05") + "\r\n" +
		fmt.Sprintf("Laptop (battery found): %t\r\n", hasBattery()) +
		"Theme: " + themeKeys[theme] + "\r\n" +
		"Language: " + langs[lang].code + "\r\n"
}

// programChecksum returns the SHA-256 line of the running Lumistar.exe, in the same format as the release files.
func programChecksum() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]) + "  Lumistar.exe\n"
}

// startupLog adds one line per start. Past about 1 MB the file moves to lumistar.log.old, so only two copies exist.
func startupLog() {
	p := filepath.Join(dataDir(), "lumistar.log")
	if fi, err := os.Stat(p); err == nil && fi.Size() > 1<<20 {
		os.Rename(p, p+".old")
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  start %s  battery=%t  theme=%s  language=%s\n",
		time.Now().Format("2006-01-02 15:04:05"), releaseText, hasBattery(), themeKeys[theme], langs[lang].code)
}

const dataReadme = `Lumistar data folder
====================

Lumistar creates this folder. The program does not need any of these files to run.

settings.txt       Theme, language and the microphone level saved by the microphone lock.
SHA256SUMS.txt     SHA-256 value of the Lumistar.exe that last ran from this PC.
README.txt         This file. It is rewritten each time Lumistar starts.
version.txt        The Lumistar version and the build it was made with.
install-path.txt   The folder of the Lumistar.exe that last ran.
last-start.txt     Time, laptop or desktop, theme and language of the last start.
lumistar.log       One line for each start. Past about 1 MB the older part moves
                   to lumistar.log.old.
brightness.log     Laptops only: why the built-in panel slider did not work.
crash.log          Only after an error: the details of that error.

To reset the settings, delete settings.txt. The uninstaller keeps this folder;
delete it yourself to remove all of Lumistar's data.
`
