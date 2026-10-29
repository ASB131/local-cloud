package names

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

var reserved = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {},
	"COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {},
	"LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

func NameKey(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r <= unicode.MaxASCII && unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func Validate(name string, allowEmpty bool) error {
	if allowEmpty && name == "" {
		return nil
	}
	if len(name) == 0 || len(name) > 255 {
		return fmt.Errorf("name length must be 1-255")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid name")
	}
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("name cannot end with space or dot")
	}
	for _, r := range name {
		if r == 0 || r < 32 {
			return fmt.Errorf("invalid character in name")
		}
		if r == '/' || r == '\\' {
			return fmt.Errorf("invalid character in name")
		}
		if strings.ContainsRune("<>:\"|?*", r) {
			return fmt.Errorf("invalid character in name")
		}
	}
	base := strings.ToUpper(name)
	if idx := strings.Index(base, "."); idx >= 0 {
		base = base[:idx]
	}
	if _, ok := reserved[base]; ok {
		return fmt.Errorf("reserved device name")
	}
	return nil
}

func SanitizeZipSegment(name string) string {
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "_"
	}
	return name
}

func ExtLower(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}
