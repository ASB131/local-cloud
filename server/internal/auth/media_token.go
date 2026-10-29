package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const mediaTokenLifetime = 15 * time.Minute

type MediaClaims struct {
	jwt.RegisteredClaims
	UserID    uuid.UUID  `json:"uid"`
	FileID    uuid.UUID  `json:"fid"`
	VersionID *uuid.UUID `json:"vid,omitempty"`
	Purpose   string     `json:"purpose"`
}

func (t *TokenIssuer) IssueMediaToken(userID, fileID uuid.UUID) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(mediaTokenLifetime)
	jti, err := uuid.NewV7()
	if err != nil {
		return "", time.Time{}, err
	}
	claims := MediaClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti.String(),
		},
		UserID:  userID,
		FileID:  fileID,
		Purpose: "media_read",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (t *TokenIssuer) IssueVersionMediaToken(userID, fileID, versionID uuid.UUID) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(mediaTokenLifetime)
	jti, err := uuid.NewV7()
	if err != nil {
		return "", time.Time{}, err
	}
	claims := MediaClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti.String(),
		},
		UserID:    userID,
		FileID:    fileID,
		VersionID: &versionID,
		Purpose:   "media_read_version",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (t *TokenIssuer) ParseMediaToken(tokenString string, fileID uuid.UUID) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MediaClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return t.secret, nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := token.Claims.(*MediaClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}
	if claims.Purpose != "media_read" || claims.FileID != fileID {
		return uuid.Nil, fmt.Errorf("invalid token scope")
	}
	return claims.UserID, nil
}

func (t *TokenIssuer) ParseVersionMediaToken(tokenString string, fileID, versionID uuid.UUID) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MediaClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return t.secret, nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := token.Claims.(*MediaClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}
	if claims.Purpose != "media_read_version" || claims.FileID != fileID || claims.VersionID == nil || *claims.VersionID != versionID {
		return uuid.Nil, fmt.Errorf("invalid token scope")
	}
	return claims.UserID, nil
}
