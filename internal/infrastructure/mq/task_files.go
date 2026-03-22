package mq

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
)

// taskFiles owns checkpoint names and source transport representations.
type taskFiles struct{ mediadomain.TranscodeTask }

func (t taskFiles) CheckpointKey() string {
	identity := t.sanitizeCheckpointToken(t.FileHash, 16)
	if identity == "" {
		sum := sha1.Sum([]byte(t.SourcePath + "|" + t.OutputPath)) //nolint:gosec // 目录名指纹
		identity = hex.EncodeToString(sum[:])[:16]
	}

	return fmt.Sprintf("v%d_p%d_%s", t.VideoID, t.VideoPartID, identity)
}

func (t taskFiles) sanitizeCheckpointToken(raw string, maxLen int) string {
	var b strings.Builder

	for _, r := range strings.TrimSpace(raw) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}

		if b.Len() >= maxLen {
			break
		}
	}

	return b.String()
}

func (t taskFiles) StagedSourceCheckpointName() string {
	sourceURL := t.SourcePath
	name := "staged-source"
	if u, err := url.Parse(sourceURL); err == nil {
		if ext := filepath.Ext(u.Path); ext != "" && len(ext) <= 10 {
			name += ext
		}
	}

	return name
}

func (t taskFiles) StagedSourceTempPattern(sourceURLPath string) string {
	pattern := "transcode-source-*"
	if ext := filepath.Ext(sourceURLPath); ext != "" && len(ext) <= 10 {
		pattern += ext
	}

	return pattern
}

func (t taskFiles) EnsureRemoteSource() error {
	u, err := url.ParseRequestURI(t.SourcePath)
	if err != nil {
		return fmt.Errorf("invalid source url: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %s", u.Scheme)
	}

	return nil
}
