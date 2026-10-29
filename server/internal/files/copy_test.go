package files

import "testing"

func TestCopyDisplayName(t *testing.T) {
	if got := copyDisplayName("report.txt", 0); got != "report - Copy.txt" {
		t.Fatalf("got %q", got)
	}
	if got := copyDisplayName("report.txt", 1); got != "report - Copy (2).txt" {
		t.Fatalf("got %q", got)
	}
	if got := copyDisplayName("Photos", 0); got != "Photos - Copy" {
		t.Fatalf("got %q", got)
	}
}
