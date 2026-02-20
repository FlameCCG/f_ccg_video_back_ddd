package mq

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

const (
	dashOutputValidationTimeout = 15 * time.Second
	maxDASHManifestBytes        = 8 << 20
)

var dashDurationPattern = regexp.MustCompile(
	`^P(?:(\d+(?:\.\d+)?)D)?(?:T(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?)?$`,
)

type dashManifestInspection struct {
	durationSec int
	videoPath   string
}

type dashManifestXML struct {
	Type                      string          `xml:"type,attr"`
	MediaPresentationDuration string          `xml:"mediaPresentationDuration,attr"`
	Periods                   []dashPeriodXML `xml:"Period"`
}

type dashPeriodXML struct {
	AdaptationSets []dashAdaptationSetXML `xml:"AdaptationSet"`
}

type dashAdaptationSetXML struct {
	ContentType     string                  `xml:"contentType,attr"`
	MimeType        string                  `xml:"mimeType,attr"`
	MaxWidth        int                     `xml:"maxWidth,attr"`
	MaxHeight       int                     `xml:"maxHeight,attr"`
	Representations []dashRepresentationXML `xml:"Representation"`
}

type dashRepresentationXML struct {
	MimeType string `xml:"mimeType,attr"`
	Width    int    `xml:"width,attr"`
	Height   int    `xml:"height,attr"`
	BaseURL  string `xml:"BaseURL"`
}

type dashVideoProbe func(context.Context, string) (media.VideoMeta, error)

func validateDASHOutput(
	ctx context.Context,
	dir string,
	sourceDuration int,
	logger *zap.Logger,
) error {
	validationCtx, cancel := context.WithTimeout(ctx, dashOutputValidationTimeout)
	defer cancel()

	probe := func(ctx context.Context, path string) (media.VideoMeta, error) {
		return ProbeVideo(ctx, path, logger)
	}

	err := validateDASHOutputWithProbe(
		validationCtx,
		dir,
		sourceDuration,
		probe,
	)
	if errors.Is(validationCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf(
			"DASH output validation timed out after %s: %w",
			dashOutputValidationTimeout,
			context.DeadlineExceeded,
		)
	}

	return err
}

func validateDASHOutputWithProbe(
	ctx context.Context,
	dir string,
	sourceDuration int,
	probe dashVideoProbe,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if probe == nil {
		return errors.New("DASH video probe is not configured")
	}

	inspection, err := inspectDASHManifest(dir)
	if err != nil {
		return err
	}

	if err := media.ValidateTranscodedDuration(sourceDuration, inspection.durationSec); err != nil {
		return fmt.Errorf("validate DASH manifest duration: %w", err)
	}

	meta, err := probe(ctx, inspection.videoPath)
	if err != nil {
		return fmt.Errorf(
			"probe DASH video representation %q: %w",
			filepath.Base(inspection.videoPath),
			err,
		)
	}

	if err := media.ValidateTranscodedDuration(sourceDuration, meta.Duration); err != nil {
		return fmt.Errorf("validate DASH video representation: %w", err)
	}

	return nil
}

func inspectDASHManifest(dir string) (dashManifestInspection, error) {
	mpdPath := filepath.Join(dir, media.DashManifestName)
	info, err := os.Stat(mpdPath)
	if err != nil {
		return dashManifestInspection{}, fmt.Errorf("stat DASH manifest: %w", err)
	}

	if !info.Mode().IsRegular() || info.Size() == 0 {
		return dashManifestInspection{}, errors.New("DASH manifest is not a non-empty regular file")
	}

	if info.Size() > maxDASHManifestBytes {
		return dashManifestInspection{}, fmt.Errorf(
			"DASH manifest exceeds %d-byte validation limit",
			maxDASHManifestBytes,
		)
	}

	data, err := os.ReadFile(mpdPath)
	if err != nil {
		return dashManifestInspection{}, fmt.Errorf("read DASH manifest: %w", err)
	}

	var manifest dashManifestXML
	if err := xml.Unmarshal(data, &manifest); err != nil {
		return dashManifestInspection{}, fmt.Errorf("parse DASH manifest XML: %w", err)
	}

	if !strings.EqualFold(strings.TrimSpace(manifest.Type), "static") {
		return dashManifestInspection{}, fmt.Errorf("DASH manifest type %q is not static", manifest.Type)
	}

	durationSec, err := parseDASHDurationSeconds(manifest.MediaPresentationDuration)
	if err != nil {
		return dashManifestInspection{}, fmt.Errorf("parse DASH manifest duration: %w", err)
	}

	inspection := dashManifestInspection{durationSec: durationSec}
	representationCount := 0

	for _, period := range manifest.Periods {
		for _, adaptationSet := range period.AdaptationSets {
			for _, representation := range adaptationSet.Representations {
				representationCount++

				repPath, err := resolveDASHRepresentationPath(dir, representation.BaseURL)
				if err != nil {
					return dashManifestInspection{}, err
				}

				if err := validateDASHRepresentationFile(repPath); err != nil {
					return dashManifestInspection{}, err
				}

				if inspection.videoPath == "" &&
					isDASHVideoRepresentation(adaptationSet, representation) {
					inspection.videoPath = repPath
				}
			}
		}
	}

	if representationCount == 0 {
		return dashManifestInspection{}, errors.New("DASH manifest has no representations")
	}

	if inspection.videoPath == "" {
		return dashManifestInspection{}, errors.New("DASH manifest has no video representation")
	}

	return inspection, nil
}

func resolveDASHRepresentationPath(dir, rawBaseURL string) (string, error) {
	baseURL := strings.TrimSpace(rawBaseURL)
	if baseURL == "" {
		return "", errors.New("DASH representation has an empty BaseURL")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse DASH representation BaseURL %q: %w", baseURL, err)
	}

	if parsed.IsAbs() || parsed.Host != "" {
		return "", fmt.Errorf("unsafe BaseURL %q: external URLs are not allowed", baseURL)
	}

	unescapedPath, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", fmt.Errorf("parse DASH representation BaseURL path %q: %w", baseURL, err)
	}

	relativePath := filepath.Clean(filepath.FromSlash(unescapedPath))
	if relativePath == "." || filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("unsafe BaseURL %q", baseURL)
	}

	fullPath := filepath.Join(dir, relativePath)
	relativeToDir, err := filepath.Rel(dir, fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve DASH representation BaseURL %q: %w", baseURL, err)
	}

	if relativeToDir == ".." || strings.HasPrefix(relativeToDir, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe BaseURL %q: path escapes DASH directory", baseURL)
	}

	return fullPath, nil
}

func validateDASHRepresentationFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat DASH representation file %q: %w", filepath.Base(path), err)
	}

	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("DASH representation file %q is empty or not regular", filepath.Base(path))
	}

	return nil
}

func isDASHVideoRepresentation(
	adaptationSet dashAdaptationSetXML,
	representation dashRepresentationXML,
) bool {
	if strings.EqualFold(strings.TrimSpace(adaptationSet.ContentType), "video") {
		return true
	}

	mimeType := strings.ToLower(strings.TrimSpace(representation.MimeType))
	if mimeType == "" {
		mimeType = strings.ToLower(strings.TrimSpace(adaptationSet.MimeType))
	}

	return strings.HasPrefix(mimeType, "video/") ||
		representation.Width > 0 ||
		representation.Height > 0 ||
		adaptationSet.MaxWidth > 0 ||
		adaptationSet.MaxHeight > 0
}

func parseDASHDurationSeconds(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	matches := dashDurationPattern.FindStringSubmatch(value)
	if len(matches) != 5 {
		return 0, fmt.Errorf("unsupported ISO-8601 duration %q", raw)
	}

	var totalSeconds float64

	hasComponent := false
	multipliers := [...]float64{24 * 60 * 60, 60 * 60, 60, 1}

	for i, multiplier := range multipliers {
		component := matches[i+1]
		if component == "" {
			continue
		}

		parsed, err := strconv.ParseFloat(component, 64)
		if err != nil {
			return 0, fmt.Errorf("parse ISO-8601 duration component %q: %w", component, err)
		}

		hasComponent = true
		totalSeconds += parsed * multiplier
	}

	invalidDuration := !hasComponent ||
		totalSeconds <= 0 ||
		math.IsInf(totalSeconds, 0) ||
		totalSeconds > float64(math.MaxInt)
	if invalidDuration {
		return 0, fmt.Errorf("invalid ISO-8601 duration %q", raw)
	}

	return int(math.Round(totalSeconds)), nil
}
