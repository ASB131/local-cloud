package files

import "testing"

func TestParseRangeFull(t *testing.T) {
	start, end, ok, bad := parseRange("", 100)
	if bad || ok || start != 0 || end != 0 {
		t.Fatalf("unexpected %d %d %v %v", start, end, ok, bad)
	}
}

func TestParseRangeSuffix(t *testing.T) {
	start, end, ok, bad := parseRange("bytes=-10", 100)
	if bad || !ok || start != 90 || end != 99 {
		t.Fatalf("got %d-%d ok=%v bad=%v", start, end, ok, bad)
	}
}

func TestParseRangeBad(t *testing.T) {
	_, _, _, bad := parseRange("bytes=0-1,2-3", 10)
	if !bad {
		t.Fatal("expected bad")
	}
}
