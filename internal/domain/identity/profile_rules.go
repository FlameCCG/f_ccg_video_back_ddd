package identity

import (
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// RegisterSourceEmail 邮箱注册来源标识（落库到 users.register_source）。
const RegisterSourceEmail = "邮箱"

// BirthdayLayout 生日的请求/响应格式。
const BirthdayLayout = "2006-01-02"

// ValidateGender 校验性别取值是否合法（未知/男/女）。
func ValidateGender(gender int) error {
	switch gender {
	case enum.GenderMale, enum.GenderFemale, enum.GenderUnknown:
		return nil
	default:
		return ErrGenderInvalid
	}
}

// ParseBirthday 解析 2006-01-02 格式的生日。
func ParseBirthday(value string) (time.Time, error) {
	birthday, err := time.Parse(BirthdayLayout, value)
	if err != nil {
		return time.Time{}, ErrBirthdayFormatInvalid
	}

	return birthday, nil
}

// FormatBirthday 把生日格式化为 2006-01-02；为空时返回空串。
func FormatBirthday(birthday *time.Time) string {
	if birthday == nil {
		return ""
	}

	return birthday.Format(BirthdayLayout)
}

// NormalizeLikeTags 去空白、去空串、去重，并保持原顺序；空集合返回空切片而非 nil。
//
// 等价原 api/user_api/user_like_tags_helper.go 的 normalizeUserLikeTags。
func NormalizeLikeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(tags))

	normalized := make([]string, 0, len(tags))

	for _, tag := range tags {
		cleaned := strings.TrimSpace(tag)
		if cleaned == "" {
			continue
		}

		if _, exists := seen[cleaned]; exists {
			continue
		}

		seen[cleaned] = struct{}{}
		normalized = append(normalized, cleaned)
	}

	if len(normalized) == 0 {
		return []string{}
	}

	return normalized
}
