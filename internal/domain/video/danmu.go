package video

import (
	"fmt"
	"strings"
	"time"
)

// 弹幕规则常量（与原 service/video_service/danmu_service.go 一致）。
const (
	// MaxDanmuLength 弹幕内容最大字符数.
	MaxDanmuLength = 200
	// DanmuRecentMax 最近弹幕缓存列表长度上限.
	DanmuRecentMax = 500
	// DanmuRecentCacheTTL 最近弹幕缓存 TTL.
	DanmuRecentCacheTTL = 2 * time.Hour
	// DefaultDanmuFontSize 默认字体大小.
	DefaultDanmuFontSize int8 = 25
	// DefaultDanmuColor 默认弹幕颜色.
	DefaultDanmuColor = "#FFFFFF"
	// DanmuHistoryDefaultLimit 历史弹幕默认返回条数.
	DanmuHistoryDefaultLimit = 200
)

// 房间弹幕列表规则（与原 danmu_room_list.go 常量一致）。
const (
	// DanmuRoomHistoryLimit 历史弹幕（非分段）固定返回最近条数.
	DanmuRoomHistoryLimit = 500
	// DanmuRoomDefaultSegmentPageSize 分段模式默认分页大小.
	DanmuRoomDefaultSegmentPageSize = 3000
	// DanmuRoomMaxSegmentPageSize 分段模式最大分页大小.
	DanmuRoomMaxSegmentPageSize = 5000
	// DanmuRoomMaxSegmentDuration 分段查询最大时间跨度（毫秒）.
	DanmuRoomMaxSegmentDuration = 60 * 1000
)

// Danmu 弹幕实体。
type Danmu struct {
	ID          uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	UserID      uint
	VideoID     uint
	VideoPartID uint
	Content     string
	TimeOffset  int
	Color       string
	FontSize    int8
	Position    int8
	LikeCount   int64
}

// OwnedBy 判断弹幕是否属于指定用户。
func (d *Danmu) OwnedBy(userID uint) bool {
	if d == nil || userID == 0 {
		return false
	}

	return d.UserID == userID
}

// SendDanmuCommand 发送弹幕的领域命令（校验并归一化后的入参）。
type SendDanmuCommand struct {
	VideoID     uint
	VideoPartID uint
	Content     string
	TimeOffset  int
	Color       string
	FontSize    int8
	Position    int8
}

// NormalizeSendDanmu 校验并归一化弹幕发送参数。
//
// 校验顺序、错误文案与默认值填充与原 DanmuService.Send 逐行一致：
// 内容裁剪空白 → 非空 → 长度 ≤200 → timeOffset ≥0 → position ∈{0,1,2}
// → fontSize 为 0 时补 25 → color 为空时补 #FFFFFF。
func NormalizeSendDanmu(userID uint, cmd SendDanmuCommand) (SendDanmuCommand, error) {
	if userID == 0 {
		return cmd, ErrLoginRequired
	}

	if cmd.VideoID == 0 {
		return cmd, ErrVideoIDRequired
	}

	content := strings.TrimSpace(cmd.Content)
	if content == "" {
		return cmd, ErrDanmuContentEmpty
	}

	if len([]rune(content)) > MaxDanmuLength {
		return cmd, fmt.Errorf("弹幕内容过长，最多%d字符", MaxDanmuLength)
	}

	cmd.Content = content

	if cmd.TimeOffset < 0 {
		return cmd, ErrDanmuTimeOffsetNegative
	}

	if cmd.Position < 0 || cmd.Position > 2 {
		return cmd, ErrDanmuPositionInvalid
	}

	if cmd.FontSize == 0 {
		cmd.FontSize = DefaultDanmuFontSize
	}

	if cmd.Color == "" {
		cmd.Color = DefaultDanmuColor
	}

	return cmd, nil
}

// DanmuPartAvailability 视频与分P可用性快照（由仓储一次查询填充）。
type DanmuPartAvailability struct {
	VideoExists bool
	PartCount   int64
	OnlyPartID  uint
	PartExists  bool
}

// ResolveDanmuPart 依据可用性快照解析实际可用的 VideoPartID（单视频返回 0）。
//
// 判定顺序与原 ResolveDanmuPart 一致：
// 视频不存在 → 报错；无分P → 0；未传 partId 且仅一个分P → 该分P；
// 未传 partId 且多分P → 报错；传了 partId 但不存在 → 报错。
func ResolveDanmuPart(partID uint, row DanmuPartAvailability) (uint, error) {
	if !row.VideoExists {
		return 0, ErrVideoNotPublished
	}

	if row.PartCount == 0 {
		return 0, nil
	}

	if partID == 0 {
		if row.PartCount == 1 && row.OnlyPartID > 0 {
			return row.OnlyPartID, nil
		}

		return 0, ErrDanmuPartIDRequired
	}

	if !row.PartExists {
		return 0, ErrDanmuPartNotFound
	}

	return partID, nil
}

// NormalizeDanmuHistoryRange 归一化历史弹幕查询的时间区间与条数上限。
//
// 与原 DanmuService.History 的裁剪逻辑一致。
func NormalizeDanmuHistoryRange(start, end, limit int) (int, int, int) {
	if start < 0 {
		start = 0
	}

	if end > 0 && end < start {
		end = start
	}

	if limit <= 0 || limit > DanmuRecentMax {
		limit = DanmuHistoryDefaultLimit
	}

	return start, end, limit
}

// NormalizeDanmuRoomSegment 校验并归一化房间弹幕的分段查询参数。
//
// 与原 DanmuService.RoomList 中 UseTimeRange 分支的校验逐行一致。
func NormalizeDanmuRoomSegment(start, end, pageSize int) (int, error) {
	if end <= 0 {
		return 0, errEndRequired
	}

	if end <= start {
		return 0, errEndMustBeGreater
	}

	if end-start > DanmuRoomMaxSegmentDuration {
		return 0, fmt.Errorf("start/end 时间范围不能超过%d毫秒", DanmuRoomMaxSegmentDuration)
	}

	if pageSize <= 0 {
		pageSize = DanmuRoomDefaultSegmentPageSize
	}

	if pageSize > DanmuRoomMaxSegmentPageSize {
		pageSize = DanmuRoomMaxSegmentPageSize
	}

	return pageSize, nil
}

// ParseDanmuDateRange 解析 YYYY-MM-DD 日期为 [start, start+24h) 区间（本地时区）。
func ParseDanmuDateRange(date string) (time.Time, time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, errDateFormat
	}

	return t, t.Add(24 * time.Hour), nil
}

// ParseDanmuMonthRange 解析 YYYY-MM 为 [月初, 下月初) 区间（本地时区）。
func ParseDanmuMonthRange(month string) (time.Time, time.Time, error) {
	t, err := time.ParseInLocation("2006-01", strings.TrimSpace(month), time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, errMonthFormat
	}

	return t, t.AddDate(0, 1, 0), nil
}

// ParseDanmuInstantRange 解析浏览器给出的绝对时间窗。
//
// 弹幕 created_at 按 Asia/Shanghai 落库。日历上的“哪一天”由浏览器时区决定，
// 所以这里只接受前端算好的月初、下月初瞬间，不使用服务器的 time.Local。
func ParseDanmuInstantRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, err := parseRFC3339(startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, errInstantRange
	}

	end, err := parseRFC3339(endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, errInstantRange
	}

	if !end.After(start) || end.Sub(start) > 32*24*time.Hour {
		return time.Time{}, time.Time{}, errInstantRange
	}

	return start, end, nil
}

func parseRFC3339(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
}
