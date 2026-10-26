package media

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"fmt"
	"math"
	"strings"
)

// defaultHighBitrateThreshold 未配置 highBitrateThreshold 时的高码率判定阈值（kbps）。
const defaultHighBitrateThreshold = 10000

// VideoMeta 描述 ffprobe 探测到的视频元数据（原 mq_service.VideoMeta）。
type VideoMeta struct {
	Height           int
	Width            int
	Bitrate          int    // kbps
	Format           string // ffprobe format_name
	Codec            string // video codec
	VideoProfile     string
	PixelFormat      string
	VideoLevel       int
	AudioCodec       string
	AudioProfile     string
	MajorBrand       string
	CompatibleBrands string
	Size             int64
	Duration         int // seconds
}

// HasAudio 报告源是否带音轨。
func (m VideoMeta) HasAudio() bool {
	return strings.TrimSpace(m.AudioCodec) != ""
}

// Resolution 按宽高映射到数据库分辨率枚举。
func (m VideoMeta) Resolution() enum.VideoResolution {
	return MapResolutionEnum(m.Width, m.Height)
}

// IsHighBitrate 判断源片是否为高码率（仅 1080P / 4K 档参与判定）。
// threshold <= 0 时使用默认阈值 10000 kbps。
func (m VideoMeta) IsHighBitrate(threshold int) bool {
	if m.Bitrate <= 0 {
		return false
	}

	resEnum := m.Resolution()
	if resEnum != enum.Resolution1080P && resEnum != enum.Resolution2160P {
		return false
	}

	if threshold <= 0 {
		threshold = defaultHighBitrateThreshold
	}

	return m.Bitrate > threshold
}

// CanonicalResourceFormat 归一化资源容器格式名，浏览器友好容器统一为 mp4。
func (m VideoMeta) CanonicalResourceFormat() string {
	if m.IsBrowserFriendlyContainer() {
		return "mp4"
	}

	return sanitizeFormat(m.Format)
}

// IsBrowserFriendlyContainer 判断容器是否浏览器友好。
//
// 注意：不要只看扩展名。很多文件名是 .mp4，但容器实际可能是 mpegts，
// 或者虽然 ffprobe 走的是 mov/mp4 demuxer，但 major_brand 实际是 qt。
func (m VideoMeta) IsBrowserFriendlyContainer() bool {
	majorBrand := strings.ToLower(strings.TrimSpace(m.MajorBrand))
	if strings.HasPrefix(majorBrand, "qt") {
		return false
	}

	compatibleBrands := strings.ToLower(strings.TrimSpace(m.CompatibleBrands))
	if strings.Contains(compatibleBrands, "qt") && !hasKnownMP4Brand(compatibleBrands) {
		return false
	}

	allBrands := strings.ToLower(majorBrand + compatibleBrands)
	if hasKnownMP4Brand(allBrands) {
		return true
	}

	format := strings.ToLower(strings.TrimSpace(m.Format))

	return strings.Contains(format, "mp4")
}

// AssessBrowserPlayableSource 判断源片能否被浏览器稳定直播放；
// 第二个返回值为不可播放的原因（可播放时为空串）。
func (m VideoMeta) AssessBrowserPlayableSource() (bool, string) {
	if !m.IsBrowserFriendlyContainer() {
		return false, fmt.Sprintf(
			"container not browser-friendly: format=%q major_brand=%q compatible_brands=%q",
			m.Format, m.MajorBrand, m.CompatibleBrands,
		)
	}

	if c := strings.ToLower(strings.TrimSpace(m.Codec)); c != "h264" {
		return false, fmt.Sprintf("video codec %q is not browser-stable h264", m.Codec)
	}

	if !isAllowedH264Profile(m.VideoProfile) {
		return false, fmt.Sprintf("video profile %q is not browser-stable", m.VideoProfile)
	}

	if !isAllowedPixelFormat(m.PixelFormat) {
		return false, fmt.Sprintf("pixel format %q is not browser-stable", m.PixelFormat)
	}

	if m.VideoLevel > 52 {
		return false, fmt.Sprintf("video level %d exceeds common browser-safe range", m.VideoLevel)
	}

	if !m.isBrowserFriendlyAudio() {
		return false, fmt.Sprintf(
			"audio codec/profile %q/%q is not browser-stable AAC-LC",
			m.AudioCodec, m.AudioProfile,
		)
	}

	return true, ""
}

func (m VideoMeta) isBrowserFriendlyAudio() bool {
	audioCodec := strings.ToLower(strings.TrimSpace(m.AudioCodec))
	if audioCodec == "" {
		return true
	}

	if audioCodec != "aac" {
		return false
	}

	switch normalizeFFprobeProfile(m.AudioProfile) {
	case "", "lc", "aac lc", "mpeg-4 aac lc":
		return true
	default:
		return false
	}
}

func sanitizeFormat(format string) string {
	if format == "" {
		return "mp4"
	}

	parts := strings.Split(format, ",")

	token := strings.TrimSpace(parts[0])
	if token == "" {
		return "mp4"
	}

	return token
}

func hasKnownMP4Brand(brands string) bool {
	for _, token := range []string{
		"isom", "iso2", "iso3", "iso4", "iso5", "iso6",
		"mp41", "mp42", "avc1", "dash", "m4v", "m4a", "f4v", "f4a",
	} {
		if strings.Contains(brands, token) {
			return true
		}
	}

	return false
}

func isAllowedH264Profile(profile string) bool {
	switch normalizeFFprobeProfile(profile) {
	case "", "baseline", "constrained baseline", "main", "high":
		return true
	default:
		return false
	}
}

func isAllowedPixelFormat(pixFmt string) bool {
	switch strings.ToLower(strings.TrimSpace(pixFmt)) {
	case "yuv420p", "yuvj420p", "nv12":
		return true
	default:
		return false
	}
}

func normalizeFFprobeProfile(profile string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(profile))), " ")
}

func lowerTrim(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// MapResolutionEnum 按宽高映射数据库分辨率枚举。
func MapResolutionEnum(width, height int) enum.VideoResolution {
	if width >= 3840 || height >= 2160 {
		return enum.Resolution2160P
	}

	if width >= 1920 || height >= 1080 {
		return enum.Resolution1080P
	}

	if width >= 1280 || height >= 720 {
		return enum.Resolution720P
	}

	if width >= 640 || height >= 360 {
		return enum.Resolution360P
	}

	return enum.Resolution360P
}

// MapResolutionEnumByHeight 按目标高度映射数据库分辨率枚举。
func MapResolutionEnumByHeight(height int) enum.VideoResolution {
	if height >= 2160 {
		return enum.Resolution2160P
	}

	if height >= 1080 {
		return enum.Resolution1080P
	}

	if height >= 720 {
		return enum.Resolution720P
	}

	return enum.Resolution360P
}

// CanonicalResolutionHeight 返回分辨率枚举对应的标准像素高度。
func CanonicalResolutionHeight(res enum.VideoResolution) uint16 {
	switch res {
	case enum.Resolution2160P:
		return 2160
	case enum.Resolution1080P, enum.Resolution1080PHigh:
		return 1080
	case enum.Resolution720P:
		return 720
	case enum.Resolution360P:
		return 360
	case enum.ResolutionAuto:
		fallthrough
	default:
		return 360
	}
}

// EffectivePixelHeight 返回用于 scale 的真实帧高。
// 优先 ffprobe 高度；禁止用「宽≥3840 就标 2160」的档位高度去上采样。
func EffectivePixelHeight(metaHeight int, classHeight uint16) uint16 {
	if metaHeight > 0 && metaHeight <= math.MaxUint16 {
		return uint16(metaHeight)
	}

	return classHeight
}

// ValidateTranscodedDuration 校验转码产物时长与源片是否吻合（2% 容差，最低 3 秒）。
func ValidateTranscodedDuration(sourceDuration, outputDuration int) error {
	if sourceDuration <= 0 {
		return nil
	}

	if outputDuration <= 0 {
		return fmt.Errorf("transcoded output duration is invalid: %d seconds", outputDuration)
	}

	tolerance := max(
		// 2% 容差，覆盖 ffprobe 取整和容器封装轻微漂移。
		sourceDuration/50, 3,
	)

	delta := sourceDuration - outputDuration
	if delta < 0 {
		delta = -delta
	}

	if delta > tolerance {
		return fmt.Errorf(
			"transcoded output duration mismatch: source=%ds output=%ds tolerance=%ds",
			sourceDuration,
			outputDuration,
			tolerance,
		)
	}

	return nil
}
