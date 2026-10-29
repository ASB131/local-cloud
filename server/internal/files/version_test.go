package files

import "testing"

func TestVersionKeepThreshold(t *testing.T) {
	const max = 104857600
	old := int64(50 * 1024 * 1024)
	newSize := int64(60 * 1024 * 1024)
	if !(old < max && newSize < max) {
		t.Fatal("setup")
	}
	big := int64(200 * 1024 * 1024)
	if big < max {
		t.Fatal("setup big")
	}
	keep := old < max && newSize < max
	if !keep {
		t.Fatal("should keep history")
	}
	keep2 := big < max && newSize < max
	if keep2 {
		t.Fatal("should not keep")
	}
}
