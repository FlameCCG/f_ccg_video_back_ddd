package mq

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	media "f_ccg_video_back_ddd/internal/domain/media"
)

const testDASHManifest = `<?xml version="1.0" encoding="utf-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011"
	type="static"
	mediaPresentationDuration="PT4.0S">
	<Period>
		<AdaptationSet contentType="video" maxWidth="628" maxHeight="360">
			<Representation mimeType="video/mp4" width="628" height="360">
				<BaseURL>stream0.mp4</BaseURL>
			</Representation>
		</AdaptationSet>
		<AdaptationSet contentType="audio">
			<Representation mimeType="audio/mp4">
				<BaseURL>stream1.mp4</BaseURL>
			</Representation>
		</AdaptationSet>
	</Period>
</MPD>`

func TestInspectDASHManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDASHFixtureFile(t, dir, media.DashManifestName, testDASHManifest)
	writeDASHFixtureFile(t, dir, "stream0.mp4", "video")
	writeDASHFixtureFile(t, dir, "stream1.mp4", "audio")

	inspection, err := inspectDASHManifest(dir)
	if err != nil {
		t.Fatalf("inspectDASHManifest() error = %v", err)
	}

	if inspection.durationSec != 4 {
		t.Fatalf("duration = %d, want 4", inspection.durationSec)
	}

	wantVideoPath := filepath.Join(dir, "stream0.mp4")
	if inspection.videoPath != wantVideoPath {
		t.Fatalf("video path = %q, want %q", inspection.videoPath, wantVideoPath)
	}
}

func TestInspectDASHManifestRejectsUnsafeOrMissingRepresentations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		manifest   string
		createFile string
		wantError  string
	}{
		{
			name:      "reject parent traversal",
			manifest:  strings.Replace(testDASHManifest, "stream0.mp4", "../stream0.mp4", 1),
			wantError: "unsafe BaseURL",
		},
		{
			name:       "reject missing representation",
			manifest:   testDASHManifest,
			createFile: "stream1.mp4",
			wantError:  "representation file",
		},
		{
			name:      "reject invalid duration",
			manifest:  strings.Replace(testDASHManifest, "PT4.0S", "forever", 1),
			wantError: "duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeDASHFixtureFile(t, dir, media.DashManifestName, tt.manifest)

			if tt.createFile != "" {
				writeDASHFixtureFile(t, dir, tt.createFile, "fixture")
			}

			_, err := inspectDASHManifest(dir)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("inspectDASHManifest() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestValidateDASHOutputProbesVideoRepresentation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDASHFixtureFile(t, dir, media.DashManifestName, testDASHManifest)
	writeDASHFixtureFile(t, dir, "stream0.mp4", "video")
	writeDASHFixtureFile(t, dir, "stream1.mp4", "audio")

	var probedPath string

	err := validateDASHOutputWithProbe(
		context.Background(),
		dir,
		4,
		func(_ context.Context, path string) (media.VideoMeta, error) {
			probedPath = path

			return media.VideoMeta{Duration: 4}, nil
		},
	)
	if err != nil {
		t.Fatalf("validateDASHOutputWithProbe() error = %v", err)
	}

	if probedPath != filepath.Join(dir, "stream0.mp4") {
		t.Fatalf("probed path = %q, want video representation", probedPath)
	}
}

func TestValidateDASHOutputHonorsContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeDASHFixtureFile(t, dir, media.DashManifestName, testDASHManifest)
	writeDASHFixtureFile(t, dir, "stream0.mp4", "video")
	writeDASHFixtureFile(t, dir, "stream1.mp4", "audio")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := validateDASHOutputWithProbe(
		ctx,
		dir,
		4,
		func(ctx context.Context, _ string) (media.VideoMeta, error) {
			<-ctx.Done()

			return media.VideoMeta{}, ctx.Err()
		},
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("validateDASHOutputWithProbe() error = %v, want deadline exceeded", err)
	}
}

func TestBuildFilterComplexKeepsCinematic4KSourcePixels(t *testing.T) {
	t.Parallel()

	targets := media.BuildDashTargets([]uint16{2160}, 1632, 2160, false)

	filter := buildFilterComplex(targets, false, false)
	if strings.Contains(filter, "scale=") || !strings.Contains(filter, "null") {
		t.Fatalf("cinematic 4K filter must not upscale source: %q", filter)
	}
}

func writeDASHFixtureFile(t *testing.T, dir, name, contents string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
		t.Fatalf("write fixture %q: %v", name, err)
	}
}
