package httpapi

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testSnapshot(context.Context) (*image.RGBA, error) {
	frame := image.NewRGBA(image.Rect(0, 0, 1080, 720))
	draw.Draw(frame, frame.Bounds(), image.NewUniform(color.RGBA{R: 200, G: 30, B: 10, A: 255}), image.Point{}, draw.Src)
	return frame, nil
}

func TestPreviewUsesCapturedFrameAndPreservesAspectRatio(t *testing.T) {
	controller := testController(t)
	response := httptest.NewRecorder()
	New(controller, testSnapshot).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/preview", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/jpeg" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("预览响应不正确: %d %v", response.Code, response.Header())
	}
	frame, err := jpeg.Decode(bytes.NewReader(response.Body.Bytes()))
	if err != nil || frame.Bounds().Size() != image.Pt(640, 426) {
		t.Fatalf("预览应等比例缩小实际画面: %v %v", frame, err)
	}
	r, g, b, _ := frame.At(320, 210).RGBA()
	if r < 190*257 || g > 40*257 || b > 20*257 {
		t.Fatal("预览没有使用捕获的屏幕像素")
	}
}

func TestPreviewReportsUnavailableWindow(t *testing.T) {
	for _, failure := range []error{errors.New("窗口未绘制"), context.Canceled, context.DeadlineExceeded} {
		capture := func(context.Context) (*image.RGBA, error) { return nil, failure }
		response := httptest.NewRecorder()
		New(testController(t), capture).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/preview", nil))
		want := http.StatusServiceUnavailable
		if errors.Is(failure, context.DeadlineExceeded) {
			want = http.StatusGatewayTimeout
		}
		if response.Code != want {
			t.Fatalf("预览失败应返回 %d，实际 %d", want, response.Code)
		}
	}
}
