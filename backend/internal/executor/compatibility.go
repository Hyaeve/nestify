package executor

import (
	"os"
	"strings"
	"time"
)

const (
	compatibilityReadInterval      = 1500 * time.Millisecond
	compatibilityBatchReadLimit    = 32
	compatibilityBatchProcessLimit = 12
)

var compatibilityReadTicker = time.NewTicker(compatibilityReadInterval)

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

func limitEntriesForMode(mode string, entries []os.DirEntry) []os.DirEntry {
	if !isCompatibilityMode(mode) {
		return entries
	}
	if len(entries) <= compatibilityBatchReadLimit {
		return entries
	}
	return entries[:compatibilityBatchReadLimit]
}

func processEntriesForMode(mode string, entries []os.DirEntry, handler func(os.DirEntry) error) error {
	if !isCompatibilityMode(mode) {
		for _, entry := range entries {
			if err := handler(entry); err != nil {
				return err
			}
		}
		return nil
	}

	for i, entry := range entries {
		if i > 0 && i%compatibilityBatchProcessLimit == 0 {
			<-compatibilityReadTicker.C
		}
		if err := handler(entry); err != nil {
			return err
		}
	}

	return nil
}
