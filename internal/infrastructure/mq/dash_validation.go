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
	ID        string                  `xml:"id,attr"`
	Bandwidth uint64                  `xml:"bandwidth,attr"`
	MimeType  string                  `xml:"mimeType,attr"`
	Width     int                     `xml:"width,attr"`
	Height    int                     `xml:"height,attr"`
	BaseURL   string                  `xml:"BaseURL"`
	Segments  *dashSegmentListXML     `xml:"SegmentList"`
	Template  *dashSegmentTemplateXML `xml:"SegmentTemplate"`
}

type dashSegmentListXML struct {
	Timescale uint64                  `xml:"timescale,attr"`
	Duration  uint64                  `xml:"duration,attr"`
	Timeline  *dashSegmentTimelineXML `xml:"SegmentTimeline"`
	URLs      []struct{}              `xml:"SegmentURL"`
}

type dashSegmentTemplateXML struct {
	Timescale      uint64                  `xml:"timescale,attr"`
	StartNumber    uint64                  `xml:"startNumber,attr"`
	Initialization string                  `xml:"initialization,attr"`
	Media          string                  `xml:"media,attr"`
	Timeline       *dashSegmentTimelineXML `xml:"SegmentTimeline"`
}

type dashSegmentTimelineXML struct {
	Entries []struct {
		Start    *uint64 `xml:"t,attr"`
		Duration uint64  `xml:"d,attr"`
		Repeat   int64   `xml:"r,attr"`
	} `xml:"S"`
}

type dashVideoProbe func(context.Context, string) (media.VideoMeta, error)

// validateDASHOutput 在独立预算内核验清单、引用文件与探测时长，不因为 FFmpeg 退出码为 0 就直接接受产物。
// 完整性检查在发布资源前执行，避免把仅写出 MPD 或时长截断的目录暴露为可播放结果。
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

	data, err := os.ReadFile(mpdPath) // #nosec G304 -- The manifest path belongs to a server-owned transcode working directory.
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

	return finishInspectDASHManifest(dir, mpdPath, manifest, durationSec)
}

const maxDASHSegments = 100000

var dashSegmentNumberPattern = regexp.MustCompile(`\$Number(?:%0([1-9][0-9]?)d)?\$`)

// validateDASHTemplateFiles 只读检查 FFmpeg 原生时间轴和全部引用文件，不改写清单。
func validateDASHTemplateFiles(dir string, rep dashRepresentationXML, sourceDuration int) error {
	template := rep.Template
	if rep.BaseURL != "" || template.Initialization == "" || template.Media == "" {
		return errors.New("unsupported DASH segment template")
	}

	validateFile := func(pattern string, number uint64) error {
		name, err := expandDASHSegmentName(pattern, rep, number)
		if err != nil {
			return err
		}

		file, err := resolveDASHRepresentationPath(dir, name)
		if err != nil {
			return err
		}

		return validateDASHRepresentationFile(file)
	}
	if err := validateFile(template.Initialization, 0); err != nil {
		return err
	}

	startNumber := template.StartNumber
	if startNumber == 0 {
		startNumber = 1
	}

	end, _, err := walkDASHTimeline(template.Timeline, func(index uint64) error {
		if startNumber > math.MaxUint64-index {
			return errors.New("DASH segment number overflows")
		}

		return validateFile(template.Media, startNumber+index)
	})
	if err != nil {
		return err
	}

	return validateDASHTimelineDuration(end, template.Timescale, sourceDuration)
}

func expandDASHSegmentName(pattern string, rep dashRepresentationXML, number uint64) (string, error) {
	name := strings.ReplaceAll(pattern, "$RepresentationID$", rep.ID)
	name = strings.ReplaceAll(name, "$Bandwidth$", strconv.FormatUint(rep.Bandwidth, 10))

	name = dashSegmentNumberPattern.ReplaceAllStringFunc(name, func(token string) string {
		match := dashSegmentNumberPattern.FindStringSubmatch(token)
		if match[1] == "" {
			return strconv.FormatUint(number, 10)
		}

		width, _ := strconv.Atoi(match[1])
		if width > 20 {
			return token
		}

		return fmt.Sprintf("%0*d", width, number)
	})
	if strings.Contains(name, "$") {
		return "", fmt.Errorf("unsupported DASH filename template %q", pattern)
	}

	return name, nil
}

// walkDASHTimeline 按 SegmentTimeline 的时间刻度展开分段并逐个访问，检查引用文件所需的序号/时间点。
// 时间刻度是媒体时间基，不能直接当成毫秒或墙上时钟；本函数只读取原生时间轴，不重写分段声明。
func walkDASHTimeline(timeline *dashSegmentTimelineXML, visit func(uint64) error) (uint64, uint64, error) {
	if timeline == nil || len(timeline.Entries) == 0 {
		return 0, 0, errors.New("DASH SegmentTimeline is missing")
	}

	var end, count uint64

	for _, entry := range timeline.Entries {
		if entry.Repeat < 0 {
			return 0, 0, errors.New("negative DASH timeline repeat")
		}

		if walkDASHTimelineCondition(entry) {
			return 0, 0, errors.New("invalid or unbounded DASH timeline entry")
		}

		if entry.Start != nil {
			if count > 0 && *entry.Start != end {
				return 0, 0, errors.New("DASH timeline contains a gap or overlap")
			}

			end = *entry.Start
		}

		repeats := uint64(entry.Repeat + 1)
		if count+repeats > maxDASHSegments || entry.Duration > (math.MaxUint64-end)/repeats {
			return 0, 0, errors.New("DASH timeline exceeds limits")
		}

		for range repeats {
			if err := visitDASHSegment(visit, count); err != nil {
				return 0, 0, err
			}

			count++
		}

		end += repeats * entry.Duration
	}

	return end, count, nil
}

func validateDASHTimelineDuration(end, scale uint64, sourceDuration int) error {
	if scale == 0 {
		scale = 1
	}

	seconds := float64(end) / float64(scale)
	if seconds >= float64(math.MaxInt) {
		return errors.New("DASH timeline duration overflows")
	}

	if err := media.ValidateTranscodedDuration(sourceDuration, int(math.Round(seconds))); err != nil {
		return fmt.Errorf("segment timeline does not cover presentation: %w", err)
	}

	return nil
}

// validateDASHSegmentCoverage 兼容读取旧的单文件清单，但拒绝明显错误的声明时长。
func validateDASHSegmentCoverage(list *dashSegmentListXML, sourceDuration int) error {
	if list == nil || len(list.URLs) == 0 {
		return nil
	}

	if list.Timeline != nil {
		end, count, err := walkDASHTimeline(list.Timeline, nil)
		if err != nil {
			return err
		}

		if count != uint64(len(list.URLs)) {
			return errors.New("DASH timeline does not match segment count")
		}

		return validateDASHTimelineDuration(end, list.Timescale, sourceDuration)
	}

	if list.Duration == 0 || list.Duration > math.MaxUint64/uint64(len(list.URLs)) {
		return errors.New("invalid DASH segment duration")
	}

	return validateDASHTimelineDuration(list.Duration*uint64(len(list.URLs)), list.Timescale, sourceDuration)
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

func walkDASHTimelineCondition(entry struct {
	Start    *uint64 "xml:\"t,attr\""
	Duration uint64  "xml:\"d,attr\""
	Repeat   int64   "xml:\"r,attr\""
},
) bool {
	return entry.Duration == 0 || entry.Repeat < 0 || entry.Repeat >= maxDASHSegments
}

func finishInspectDASHManifest(dir, mpdPath string, manifest dashManifestXML, durationSec int) (dashManifestInspection, error) {
	inspection := dashManifestInspection{durationSec: durationSec}
	representationCount := 0

	for _, period := range manifest.Periods {
		for _, set := range period.AdaptationSets {
			for _, rep := range set.Representations {
				representationCount++

				candidate, err := inspectDASHRepresentation(dir, mpdPath, set, rep, durationSec)
				if err != nil {
					return dashManifestInspection{}, err
				}

				if inspection.videoPath == "" {
					inspection.videoPath = candidate
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

func visitDASHSegment(visit func(uint64) error, index uint64) error {
	if visit == nil {
		return nil
	}

	return visit(index)
}

func inspectDASHRepresentation(dir, mpdPath string, set dashAdaptationSetXML, rep dashRepresentationXML, durationSec int) (string, error) {
	if rep.Template != nil {
		if err := validateDASHTemplateFiles(dir, rep, durationSec); err != nil {
			return "", fmt.Errorf("DASH representation %q: %w", rep.ID, err)
		}

		if isDASHVideoRepresentation(set, rep) {
			return mpdPath, nil
		}

		return "", nil
	}

	if err := validateDASHSegmentCoverage(rep.Segments, durationSec); err != nil {
		return "", fmt.Errorf("DASH representation %q: %w", rep.BaseURL, err)
	}

	repPath, err := resolveDASHRepresentationPath(dir, rep.BaseURL)
	if err != nil {
		return "", err
	}

	if err := validateDASHRepresentationFile(repPath); err != nil {
		return "", err
	}

	if isDASHVideoRepresentation(set, rep) {
		return repPath, nil
	}

	return "", nil
}
