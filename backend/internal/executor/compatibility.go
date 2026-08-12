package executor

import (
	"os"
	"strings"
	"time"
)

const (
	compatibilityOperationInterval = 2 * time.Second
	compatibilityBatchProcessLimit = 8
	compatibilityBatchCooldown     = 5 * time.Second
)

var compatibilityPacer = make(chan struct{}, 1)

func isCompatibilityMode(mode string) bool {
	return strings.TrimSpace(mode) == "compatibility"
}

func readDirWithMode(mode, path string) ([]os.DirEntry, error) {
	paceCompatibilityMode(mode)
	return os.ReadDir(path)
}

func statWithMode(mode, path string) (os.FileInfo, error) {
	paceCompatibilityMode(mode)
	return os.Stat(path)
}

func entryInfoWithMode(mode string, entry os.DirEntry) (os.FileInfo, error) {
	paceCompatibilityMode(mode)
	return entry.Info()
}

func fileOperationWithMode(mode string) {
	paceCompatibilityMode(mode)
}

func limitEntriesForMode(mode string, entries []os.DirEntry) []os.DirEntry {
	return entries
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
			time.Sleep(compatibilityBatchCooldown)
		}
		paceCompatibilityMode(mode)
		if err := handler(entry); err != nil {
			return err
		}
	}

	return nil
}

func paceCompatibilityMode(mode string) {
	if !isCompatibilityMode(mode) {
		return
	}

	compatibilityPacer <- struct{}{}
	time.Sleep(compatibilityOperationInterval)
	<-compatibilityPacer
}
