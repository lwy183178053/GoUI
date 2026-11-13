package ui

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
)

// GIFSegments 用原始 GIF 的时间轴表示分段。
// Start == End 时停在该位置；否则循环播放 [Start, End)，之后的部分用于回正。
type GIFSegments struct {
	Start time.Duration
	End   time.Duration
}

// parseGIFSegments 只在显式设置 playback="segmented" 时解析文件名。
// 秒格式如 _[0.5s,1.5s]；帧格式如 _[12,37]，帧索引从 0 开始且包含末帧。
func parseGIFSegments(src string, animation *GIFAnimation) (*GIFSegments, error) {
	name := strings.TrimSuffix(path.Base(src), path.Ext(src))
	start := strings.LastIndex(name, "_[")
	if start < 0 || !strings.HasSuffix(name, "]") {
		return nil, fmt.Errorf("segmented GIF filename must end with _[time] or _[start,end]")
	}
	values := strings.Split(name[start+2:len(name)-1], ",")
	if len(values) > 2 {
		return nil, fmt.Errorf("GIF segment must contain one or two positions")
	}
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	duration := animation.Duration()
	seconds := strings.HasSuffix(values[0], "s")
	points := make([]time.Duration, len(values))
	for i, value := range values {
		if strings.HasSuffix(value, "s") != seconds {
			return nil, fmt.Errorf("GIF segment positions must use the same unit: seconds or frame indices")
		}
		if seconds {
			number := strings.TrimSpace(strings.TrimSuffix(value, "s"))
			// ParseFloat 排除混合单位，ParseDuration 负责小数秒和溢出校验。
			if _, err := strconv.ParseFloat(number, 64); err != nil {
				return nil, fmt.Errorf("invalid GIF segment time %q", value)
			}
			point, err := time.ParseDuration(number + "s")
			if err != nil || point < 0 || point > duration {
				return nil, fmt.Errorf("GIF segment time %q must be between 0s and %s", value, duration)
			}
			points[i] = point
		} else {
			index, err := strconv.Atoi(value)
			if err != nil || index < 0 || index >= len(animation.Frames) {
				return nil, fmt.Errorf("GIF segment frame %q must be between 0 and %d", value, len(animation.Frames)-1)
			}
			for _, frame := range animation.Frames[:index] {
				points[i] += frame.Delay
			}
			if i == 1 {
				points[i] += animation.Frames[index].Delay
			}
		}
	}
	segments := &GIFSegments{Start: points[0], End: points[0]}
	if len(points) == 2 {
		segments.End = points[1]
		if segments.End <= segments.Start {
			return nil, fmt.Errorf("GIF loop end must follow its start")
		}
	}
	return segments, nil
}
