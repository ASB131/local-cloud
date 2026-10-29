package storage

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestRelPathRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	layout, err := NewLayout(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := layout.RelPath("../etc/passwd"); err == nil {
		t.Fatal("expected error for traversal")
	}
}

func TestJoinUnderUserStaysInTree(t *testing.T) {
	dir := t.TempDir()
	layout, err := NewLayout(dir)
	if err != nil {
		t.Fatal(err)
	}
	uid := uuid.MustParse("018f1234-5678-7890-abcd-ef1234567890")
	rel, err := layout.JoinUnderUser(uid, "objects", uid.String(), "v1")
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := filepath.ToSlash(filepath.Join("users", uid.String()))
	if rel != wantPrefix+"/objects/"+uid.String()+"/v1" {
		t.Fatalf("got %q", rel)
	}
}
