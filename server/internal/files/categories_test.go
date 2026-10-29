package files

import (
	"strings"
	"testing"
)

func TestCategoryWhereUsesFileTableAlias(t *testing.T) {
	for _, c := range fileCategoryDefs {
		scrub := c.Where
		scrub = strings.ReplaceAll(scrub, "f.mime_type", "")
		scrub = strings.ReplaceAll(scrub, "f.name", "")
		if strings.Contains(scrub, "mime_type") || strings.Contains(scrub, "name") {
			t.Fatalf("category %q WHERE must qualify file columns with f.: %s", c.ID, c.Where)
		}
	}
}

func TestCategoryWhereValidForAllCategories(t *testing.T) {
	for _, c := range fileCategoryDefs {
		if _, err := categoryWhere(c.ID); err != nil {
			t.Fatalf("category %q: %v", c.ID, err)
		}
	}
	if _, err := categoryWhere("not-a-category"); err != ErrValidation {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}
