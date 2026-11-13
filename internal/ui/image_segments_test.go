package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGIFSegmentPositions(t *testing.T) {
	animation := &GIFAnimation{Frames: []GIFFrame{
		{Delay: 100 * time.Millisecond}, {Delay: 200 * time.Millisecond},
		{Delay: 300 * time.Millisecond}, {Delay: 400 * time.Millisecond},
	}}
	for _, test := range []struct {
		name       string
		start, end time.Duration
	}{
		{"eyes_[0.2s].gif", 200, 200},
		{"gif/eyes_[0.15s, 0.65s].gif", 150, 650},
		{"eyes_[1.0s].gif", 1000, 1000}, // 保持点可以正好落在 GIF 末尾。
		{"eyes_[0 s, 1.0 s].gif", 0, 1000},
		{"eyes_[0].gif", 0, 0},
		{"eyes_[1].gif", 100, 100},
		{"eyes_[1,2].gif", 100, 600}, // 帧格式包含末帧，并按各帧真实时长求和。
		{"eyes_[2,2].gif", 300, 600},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseGIFSegments(test.name, animation)
			want := GIFSegments{Start: test.start * time.Millisecond, End: test.end * time.Millisecond}
			if err != nil || *got != want {
				t.Fatalf("分段 = %v, %v，期望 %v", got, err, want)
			}
		})
	}
	for _, name := range []string{
		"eyes.gif", "eyes_[].gif", "eyes_[0.5s.gif", "eyes_[0s]_extra.gif",
		"eyes_[0s,0.5s,1s].gif", "eyes_[-1s].gif", "eyes_[2s].gif",
		"eyes_[0.5s,0.2s].gif", "eyes_[0.5s,0.5s].gif", "eyes_[0s,1].gif",
		"eyes_[NaNs].gif", "eyes_[Infs].gif", "eyes_[1h0s].gif", "eyes_[100ms].gif",
		"eyes_[-1].gif", "eyes_[4].gif", "eyes_[2,1].gif", "eyes_[1.5].gif",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGIFSegments(name, animation); err == nil {
				t.Fatal("非法文件名应报错")
			}
		})
	}
}

func TestImagePlaybackLoadingAndLocation(t *testing.T) {
	palette := color.Palette{color.Black, color.White}
	pixel := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	var gifData, singleData, pngData bytes.Buffer
	if err := gif.EncodeAll(&gifData, &gif.GIF{
		Image: []*image.Paletted{pixel, pixel}, Delay: []int{40, 60}, LoopCount: 0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(&singleData, &gif.GIF{Image: []*image.Paletted{pixel}, Delay: []int{100}}); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&pngData, pixel); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, playback string
		data           []byte
		valid          bool
		segmented      bool
	}{
		{"eyes_[0.5s,1s].gif", "segmented", gifData.Bytes(), true, true},
		{"eyes_[1s].gif", "segmented", singleData.Bytes(), true, true},
		{"eyes_[9s].gif", "auto", gifData.Bytes(), true, false},
		{"eyes_[9s].gif", "segmented", gifData.Bytes(), false, false},
		{"eyes.gif", "segmented", gifData.Bytes(), false, false},
		{"eyes_[0s].png", "segmented", pngData.Bytes(), false, false},
		{"eyes.gif", "unknown", gifData.Bytes(), false, false},
	} {
		t.Run(test.name+"/"+test.playback, func(t *testing.T) {
			dir := fixture(t, testConfig, fmt.Sprintf("<ui>\n  <image id=\"gif\" size=\"1,1\" src=\"%s\" playback=\"%s\"/>\n</ui>", test.name, test.playback))
			if err := os.WriteFile(filepath.Join(dir, test.name), test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			doc, err := Load(dir)
			if !test.valid {
				if err == nil || !strings.Contains(err.Error(), "main.xml:2:") {
					t.Fatalf("非法播放配置应包含 XML 位置：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			animation := doc.Find("gif").Image.Animation
			if animation == nil || (animation.Segments != nil) != test.segmented {
				t.Fatalf("分段配置未加载：%+v", animation)
			}
		})
	}
}
