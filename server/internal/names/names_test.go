package names

import "testing"

func TestNameKeyASCIIFold(t *testing.T) {
	if NameKey("Hello.TXT") != "hello.txt" {
		t.Fatalf("got %q", NameKey("Hello.TXT"))
	}
	if NameKey("über") != "über" {
		t.Fatalf("non-ascii case preserved")
	}
}

func TestValidateRejectsTraversal(t *testing.T) {
	if Validate("../x", false) == nil {
		t.Fatal("expected error")
	}
}
