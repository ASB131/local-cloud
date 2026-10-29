package files

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"sync"

	"github.com/google/uuid"
)

type uploadHasher struct {
	h     hash.Hash
	bytes int64
}

var uploadHashers sync.Map

func resetUploadHasher(sessionID uuid.UUID) *uploadHasher {
	state := &uploadHasher{h: sha256.New()}
	uploadHashers.Store(sessionID, state)
	return state
}

func getUploadHasher(sessionID uuid.UUID) (*uploadHasher, bool) {
	v, ok := uploadHashers.Load(sessionID)
	if !ok {
		return nil, false
	}
	return v.(*uploadHasher), true
}

func (u *uploadHasher) Write(p []byte) {
	u.h.Write(p)
	u.bytes += int64(len(p))
}

func (u *uploadHasher) SumHex() string {
	return hex.EncodeToString(u.h.Sum(nil))
}

func dropUploadHasher(sessionID uuid.UUID) {
	uploadHashers.Delete(sessionID)
}
