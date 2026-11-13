package software

import (
	"image"
	"strings"
	"unicode"

	"github.com/lwy183178053/GoUI/internal/ui"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

func paintText(dst *image.RGBA, rect image.Rectangle, style *ui.TextStyle, face font.Face, limit image.Rectangle) {
	clip := rect.Intersect(dst.Bounds()).Intersect(limit)
	if clip.Empty() || style.Color.A == 0 || style.Content == "" {
		return
	}
	lines := textLines(style.Content, face, rect.Dx(), style.Wrap)
	metrics := face.Metrics()
	lineHeight := metrics.Height
	if lineHeight <= 0 {
		lineHeight = max(fixed.I(1), metrics.Ascent+metrics.Descent)
	}
	// 多行溢出时按对齐方向保留完整行，避免底部字幕露出上方的半行文字。
	// 高度不足一行时仍保留一行，由节点矩形裁切。
	count := max(1, int(fixed.I(rect.Dy())/lineHeight))
	if len(lines) > count {
		start := 0
		switch style.VAlign {
		case "middle":
			start = (len(lines) - count) / 2
		case "bottom":
			start = len(lines) - count
		}
		lines = lines[start : start+count]
	}
	top := fixed.I(rect.Min.Y)
	freeHeight := fixed.I(rect.Dy()) - lineHeight*fixed.Int26_6(len(lines))
	switch style.VAlign {
	case "middle":
		top += freeHeight / 2
	case "bottom":
		top += freeHeight
	}
	drawer := font.Drawer{Dst: dst.SubImage(clip).(*image.RGBA), Src: image.NewUniform(style.Color), Face: face}
	for i, line := range lines {
		x := fixed.I(rect.Min.X)
		freeWidth := fixed.I(rect.Dx()) - font.MeasureString(face, line)
		switch style.Align {
		case "center":
			x += freeWidth / 2
		case "right":
			x += freeWidth
		}
		drawer.Dot = fixed.Point26_6{X: x, Y: top + metrics.Ascent + lineHeight*fixed.Int26_6(i)}
		drawer.DrawString(line)
	}
}

// textLines 根据字体实际宽度排版，而不是按字符数量估算。
// 只有启用换行时，文本中的换行符才会分段。
func textLines(content string, face font.Face, width int, wrap bool) []string {
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	content = strings.ReplaceAll(content, "\t", " ")
	if !wrap {
		return []string{strings.ReplaceAll(content, "\n", " ")}
	}
	var lines []string
	limit := fixed.I(width)
	for _, paragraph := range strings.Split(content, "\n") {
		var line string
		start := len(lines)
		flush := func() {
			lines = append(lines, strings.TrimRightFunc(line, unicode.IsSpace))
			line = ""
		}
		for _, token := range textTokens(paragraph) {
			space := strings.TrimSpace(token) == ""
			if line == "" && space {
				continue
			}
			if line != "" && font.MeasureString(face, line+token) > limit {
				flush()
				if space {
					continue
				}
			}
			if font.MeasureString(face, token) <= limit {
				line += token
				continue
			}
			// 单词过长时逐字拆分；单个字过宽也会保留，绘制时再裁切。
			for _, char := range token {
				part := string(char)
				if line != "" && font.MeasureString(face, line+part) > limit {
					flush()
				}
				line += part
			}
		}
		if line != "" || len(lines) == start {
			flush()
		}
	}
	return lines
}

func textTokens(content string) []string {
	var result []string
	var word strings.Builder
	flush := func() {
		if word.Len() != 0 {
			result = append(result, word.String())
			word.Reset()
		}
	}
	for _, char := range content {
		if unicode.IsSpace(char) || unicode.In(char, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			flush()
			result = append(result, string(char))
		} else {
			word.WriteRune(char)
		}
	}
	flush()
	return result
}
