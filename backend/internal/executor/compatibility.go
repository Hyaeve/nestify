package executor

import (
	"os"
	"strings"
	"time"
)

var compatibilityReadTicker = time.NewTicker(time.Second / 3)

func isCompatibilityMode(mode string) bool {
	return strings.TrimSpace(mode) == "compatibility"
}

func readDirWithMode(mode, path string) ([]os.DirEntry, error) {
	if isCompatibilityMode(mode) {
		<-compatibilityReadTicker.C
	}
	return os.ReadDir(path)
}

func statWithMode(mode, path string) (os.FileInfo, error) {
	if isCompatibilityMode(mode) {
		<-compatibilityReadTicker.C
	}
	return os.Stat(path)
}

func entryInfoWithMode(mode string, entry os.DirEntry) (os.FileInfo, error) {
	if isCompatibilityMode(mode) {
		<-compatibilityReadTicker.C
	}
	return entry.Info()
}
