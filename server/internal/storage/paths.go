package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Layout struct {
	root string
}

func NewLayout(dataDir string) (*Layout, error) {
	root, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	return &Layout{root: root}, nil
}

func (l *Layout) Root() string {
	return l.root
}

func (l *Layout) EnsureBaseDirs() error {
	dirs := []string{
		l.root,
		filepath.Join(l.root, "users"),
		filepath.Join(l.root, "backups"),
		filepath.Join(l.root, "tmp"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func (l *Layout) EnsureUserDirs(userID uuid.UUID) error {
	dirs := []string{
		l.UserRoot(userID),
		l.UserObjectsDir(userID),
		l.UserTmpDir(userID),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func (l *Layout) UserRoot(userID uuid.UUID) string {
	return filepath.Join(l.root, "users", userID.String())
}

func (l *Layout) UserObjectsDir(userID uuid.UUID) string {
	return filepath.Join(l.UserRoot(userID), "objects")
}

func (l *Layout) UserTmpDir(userID uuid.UUID) string {
	return filepath.Join(l.UserRoot(userID), "tmp")
}

// RelPath returns a path relative to the data root after validating it stays under root.
func (l *Layout) RelPath(rel string) (string, error) {
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes data root")
	}
	abs := filepath.Join(l.root, clean)
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(l.root)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes data root")
	}
	relOut, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relOut), nil
}

func (l *Layout) JoinUnderUser(userID uuid.UUID, parts ...string) (string, error) {
	base := l.UserRoot(userID)
	all := append([]string{base}, parts...)
	joined := filepath.Join(all...)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(l.root)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes data root")
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}
