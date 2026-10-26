package storage

import (
	"context"
	"crypto/md5" //nolint:gosec // Legacy non-authentication digest format must remain compatible with stored identifiers.
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"f_ccg_video_back_ddd/pkg/uploadscope"
	"hash"
	"strings"
)

type uploadHashVerifier struct {
	hash.Hash
	expected string
}

func newUploadHashVerifier(ctx context.Context, expected string) (*uploadHashVerifier, error) {
	var h hash.Hash

	switch len(expected) {
	case 32:
		//nolint:gosec // Compatibility checksum only; authentication and passwords use stronger primitives.
		h = md5.New()
	case 64:
		h = sha256.New()
	}

	_, err := hex.DecodeString(expected)
	if h == nil || err != nil {
		if uploadscope.HasOwner(ctx) {
			return nil, errors.New("fileHash must be an MD5 or SHA256 digest")
		}

		return &uploadHashVerifier{}, nil
	}

	return &uploadHashVerifier{Hash: h, expected: expected}, nil
}

func (v *uploadHashVerifier) Write(p []byte) (int, error) {
	if v.Hash == nil {
		return len(p), nil
	}

	return v.Hash.Write(p)
}

func (v *uploadHashVerifier) verify() error {
	if v.Hash != nil && !strings.EqualFold(hex.EncodeToString(v.Sum(nil)), v.expected) {
		return errors.New("uploaded content does not match fileHash")
	}

	return nil
}
