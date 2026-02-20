package media

import (
	"strconv"
	"strings"
)

// InjectFFmpegProgressArgs 确保 ffmpeg 参数里带上 `-progress pipe:2`，
// 使其即便在 `-loglevel error` 下也向 stderr 输出 key=value 进度。幂等。
func InjectFFmpegProgressArgs(args []string) []string {
	for i := range args {
		if args[i] == "-progress" {
			return args
		}
	}

	// 没有任何参数时直接给出进度参数。
	if len(args) == 0 {
		return []string{"-progress", "pipe:2"}
	}

	// args[0] 是程序名（exec.Command 会把 Path 放到 Args[0]），插在其后。
	out := make([]string, 0, len(args)+2)
	out = append(out, args[0], "-progress", "pipe:2")
	out = append(out, args[1:]...)

	return out
}

// ParseFFmpegProgressLine 解析 `-progress` 输出的 key=value 行，返回 out_time 秒数。
func ParseFFmpegProgressLine(line string) (float64, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, false
	}

	const outTimeMS = "out_time_ms="
	if strings.HasPrefix(line, outTimeMS) {
		v := strings.TrimPrefix(line, outTimeMS)

		ms, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil || ms < 0 {
			return 0, false
		}

		return float64(ms) / 1000.0, true
	}

	const outTimeUS = "out_time_us="
	if strings.HasPrefix(line, outTimeUS) {
		v := strings.TrimPrefix(line, outTimeUS)

		us, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil || us < 0 {
			return 0, false
		}

		return float64(us) / 1_000_000.0, true
	}

	const outTime = "out_time="
	if strings.HasPrefix(line, outTime) {
		return ParseFFmpegClock(strings.TrimPrefix(line, outTime))
	}

	return 0, false
}

// ParseFFmpegTimeEquals 解析经典 stderr 行里的 `time=HH:MM:SS.xx` 字段。
func ParseFFmpegTimeEquals(line string) (float64, bool) {
	// 例如 frame= 123 fps=... time=00:01:02.34 bitrate=...
	idx := strings.Index(line, "time=")
	if idx < 0 {
		return 0, false
	}

	rest := line[idx+len("time="):]

	field := rest
	if sp := strings.IndexAny(rest, " \t"); sp >= 0 {
		field = rest[:sp]
	}

	return ParseFFmpegClock(field)
}

// ParseFFmpegClock 解析 `HH:MM:SS.micro` 或纯秒数。
func ParseFFmpegClock(v string) (float64, bool) {
	v = strings.TrimSpace(v)
	if v == "" || v == "N/A" {
		return 0, false
	}

	parts := strings.Split(v, ":")
	if len(parts) != 3 {
		// 纯秒数
		sec, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}

		return sec, true
	}

	h, err1 := strconv.ParseFloat(parts[0], 64)
	m, err2 := strconv.ParseFloat(parts[1], 64)
	s, err3 := strconv.ParseFloat(parts[2], 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}

	return h*3600 + m*60 + s, true
}
