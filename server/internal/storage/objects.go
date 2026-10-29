package storage

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
)

func (l *Layout) ObjectDir(userID, fileID uuid.UUID) string {
	return filepath.Join(l.UserObjectsDir(userID), fileID.String())
}

func (l *Layout) VersionFile(userID, fileID uuid.UUID, versionNum int) (abs string, rel string, err error) {
	rel, err = l.JoinUnderUser(userID, "objects", fileID.String(), fmt.Sprintf("v%d", versionNum))
	if err != nil {
		return "", "", err
	}
	return filepath.Join(l.root, filepath.FromSlash(rel)), rel, nil
}

func (l *Layout) ThumbFile(userID, fileID uuid.UUID) (abs string, rel string, err error) {
	rel, err = l.JoinUnderUser(userID, "objects", fileID.String(), "thumb.jpg")
	if err != nil {
		return "", "", err
	}
	return filepath.Join(l.root, filepath.FromSlash(rel)), rel, nil
}

func (l *Layout) OfficePreviewPDF(userID, fileID uuid.UUID) (abs string, rel string, err error) {
	rel, err = l.JoinUnderUser(userID, "objects", fileID.String(), "office-preview.pdf")
	if err != nil {
		return "", "", err
	}
	return filepath.Join(l.root, filepath.FromSlash(rel)), rel, nil
}

func (l *Layout) UploadTempFile(userID uuid.UUID, sessionID uuid.UUID) (abs string, rel string, err error) {
	rel, err = l.JoinUnderUser(userID, "tmp", sessionID.String()+".part")
	if err != nil {
		return "", "", err
	}
	return filepath.Join(l.root, filepath.FromSlash(rel)), rel, nil
}

func (l *Layout) AbsFromRel(rel string) (string, error) {
	clean, err := l.RelPath(rel)
	if err != nil {
		return "", err
	}
	return filepath.Join(l.root, filepath.FromSlash(clean)), nil
}
