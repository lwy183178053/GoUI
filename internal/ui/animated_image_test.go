package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecodeGIFComposesFramesAndKeepsDelays(t *testing.T) {
	palette := color.Palette{
		color.NRGBA{},
		color.NRGBA{R: 255, A: 255},
		color.NRGBA{B: 255, A: 255},
		color.NRGBA{G: 255, A: 255},
	}
	first := image.NewPaletted(image.Rect(0, 0, 2, 1), palette)
	first.Pix[0], first.Pix[1] = 1, 1
	second := image.NewPaletted(image.Rect(1, 0, 2, 1), palette)
	second.Pix[0] = 2
	third := image.NewPaletted(image.Rect(1, 0, 2, 1), palette)
	third.Pix[0] = 3

	var encoded bytes.Buffer
	err := gif.EncodeAll(&encoded, &gif.GIF{
		Image:           []*image.Paletted{first, second, third},
		Delay:           []int{2, 3, 4},
		Disposal:        []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalNone},
		LoopCount:       0,
		BackgroundIndex: 0,
		Config:          image.Config{ColorModel: palette, Width: 2, Height: 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	animation, err := decodeGIF(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(animation.Frames) != 3 || animation.LoopCount != 0 {
		t.Fatalf("decoded %d frames with loop count %d", len(animation.Frames), animation.LoopCount)
	}
	if animation.Frames[0].Delay != 20*time.Millisecond || animation.Frames[1].Delay != 30*time.Millisecond || animation.Frames[2].Delay != 40*time.Millisecond {
		t.Fatalf("frame delays = %v, want 20ms, 30ms, 40ms", []time.Duration{animation.Frames[0].Delay, animation.Frames[1].Delay, animation.Frames[2].Delay})
	}
	player := animation.NewPlayer()
	if _, err := player.Frame(0); err != nil {
		t.Fatal(err)
	}
	secondFrame, err := player.Frame(1)
	if err != nil {
		t.Fatal(err)
	}
	wantRed := color.NRGBA{R: 255, A: 255}
	if got := color.NRGBAModel.Convert(secondFrame.At(0, 0)); got != wantRed {
		t.Fatalf("frame 2 kept pixel = %#v, want red", got)
	}
	wantBlue := color.NRGBA{B: 255, A: 255}
	if got := color.NRGBAModel.Convert(secondFrame.At(1, 0)); got != wantBlue {
		t.Fatalf("frame 2 changed pixel = %#v, want blue", got)
	}
	thirdFrame, err := player.Frame(2)
	if err != nil {
		t.Fatal(err)
	}
	wantGreen := color.NRGBA{G: 255, A: 255}
	if got := color.NRGBAModel.Convert(thirdFrame.At(1, 0)); got != wantGreen {
		t.Fatalf("frame 3 pixel = %#v, want green after background disposal", got)
	}
}

func TestLoadImageKeepsAnimatedGIFFrames(t *testing.T) {
	palette := color.Palette{color.NRGBA{A: 255}, color.NRGBA{R: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	second := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	second.Pix[0] = 1
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{
		Image:     []*image.Paletted{first, second},
		Delay:     []int{5, 5},
		LoopCount: 0,
		Config:    image.Config{ColorModel: palette, Width: 1, Height: 1},
	}); err != nil {
		t.Fatal(err)
	}

	dir := fixture(t, testConfig, `<ui><image id="eyes" size="10,10" src="eyes.gif" /></ui>`)
	if err := os.WriteFile(filepath.Join(dir, "eyes.gif"), encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	imageStyle := doc.Find("eyes").Image
	if imageStyle.Animation == nil || len(imageStyle.Animation.Frames) != 2 || imageStyle.Animation.LoopCount != 0 {
		t.Fatalf("GIF playback data was not loaded: %#v", imageStyle)
	}
}

// 三帧只更新局部像素，第三帧必须先清除第二帧留下的蓝色。
func TestGIFTransparentBackgroundAndDisposal(t *testing.T) {
	palette := color.Palette{color.NRGBA{}, color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}, color.NRGBA{G: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 2, 1), palette)
	first.Pix[0] = 1
	second := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	second.Pix[0] = 2
	third := image.NewPaletted(image.Rect(1, 0, 2, 1), palette)
	third.Pix[0] = 3
	animation := decodeTestGIF(t, &gif.GIF{
		Image: []*image.Paletted{first, second, third}, Delay: []int{1, 1, 1},
		Disposal: []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalNone},
		Config:   image.Config{ColorModel: palette, Width: 2, Height: 1},
	})
	player := animation.NewPlayer()
	frame, err := player.Frame(0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := frame.At(1, 0).RGBA(); alpha != 0 {
		t.Fatal("transparent GIF background became opaque")
	}
	frame, err = player.Frame(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := frame.At(0, 0).RGBA(); alpha != 0 {
		t.Fatal("background disposal did not clear previous frame")
	}
	if got := color.NRGBAModel.Convert(frame.At(1, 0)); got != (color.NRGBA{G: 255, A: 255}) {
		t.Fatalf("third frame = %v, want green", got)
	}
}

func TestGIFPreviousDisposalAndRestart(t *testing.T) {
	palette := color.Palette{color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}, color.NRGBA{G: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 2, 1), palette)
	second := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	second.Pix[0] = 1
	third := image.NewPaletted(image.Rect(1, 0, 2, 1), palette)
	third.Pix[0] = 2
	animation := decodeTestGIF(t, &gif.GIF{
		Image: []*image.Paletted{first, second, third}, Delay: []int{1, 1, 1},
		Disposal: []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalNone},
		Config:   image.Config{ColorModel: palette, Width: 2, Height: 1},
	})
	player := animation.NewPlayer()
	frame, err := player.Frame(2)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(frame.At(0, 0)); got != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("previous disposal = %v, want original red", got)
	}
	frame, err = player.Frame(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(frame.At(1, 0)); got != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("restarted frame = %v, want original red", got)
	}
}

func TestSingleFrameGIFAndZeroDelay(t *testing.T) {
	palette := color.Palette{color.NRGBA{}, color.NRGBA{R: 255, A: 255}}
	frame := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	var data bytes.Buffer
	if err := gif.EncodeAll(&data, &gif.GIF{Image: []*image.Paletted{frame}, Delay: []int{0}}); err != nil {
		t.Fatal(err)
	}
	animation, err := decodeGIF(data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if animation.Frames[0].Delay != 10*time.Millisecond {
		t.Fatal("zero delay must have a positive playback interval")
	}
	dir := fixture(t, testConfig, `<ui><image id="single" size="1,1" src="single.gif"/></ui>`)
	if err := os.WriteFile(filepath.Join(dir, "single.gif"), data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	style := doc.Find("single").Image
	if style.Animation != nil || style.Image == nil {
		t.Fatal("single-frame GIF must remain a static image")
	}
	if _, _, _, alpha := style.Image.At(0, 0).RGBA(); alpha != 0 {
		t.Fatal("single-frame GIF lost transparency")
	}
}

func TestInvalidGIFReportsXMLLocation(t *testing.T) {
	dir := fixture(t, testConfig, `<ui><image size="1,1" src="broken.gif"/></ui>`)
	if err := os.WriteFile(filepath.Join(dir, "broken.gif"), []byte("GIF89a"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "main.xml:1:") || !strings.Contains(err.Error(), "broken.gif") {
		t.Fatalf("invalid GIF error has no XML location or resource name: %v", err)
	}
}

func decodeTestGIF(t *testing.T, value *gif.GIF) *GIFAnimation {
	t.Helper()
	var data bytes.Buffer
	if err := gif.EncodeAll(&data, value); err != nil {
		t.Fatal(err)
	}
	animation, err := decodeGIF(data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return animation
}
