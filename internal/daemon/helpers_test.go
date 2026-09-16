package daemon

import (
	"os"
	"path/filepath"
	"strings"
)

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
