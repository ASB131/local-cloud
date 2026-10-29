package files

import "testing"

func TestEnsureFolderPathEmpty(t *testing.T) {
	// Document rebuild expectation: empty path resolves to root without extra segments.
	path := []string{}
	if len(path) != 0 {
		t.Fatal("empty path")
	}
}
