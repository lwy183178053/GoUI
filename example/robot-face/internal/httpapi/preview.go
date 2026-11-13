package httpapi

import (
	"bytes"
	"context"
	"image"
	"image/draw"
	"image/jpeg"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	xdraw "golang.org/x/image/draw"
)

// registerPreview 返回实际 UI 最近一帧，浏览器不再自行解码另一份 GIF。
// 缩小和 JPEG 编码在 HTTP goroutine 中完成，不占用窗口主循环。
func registerPreview(router *gin.Engine, capture func(context.Context) (*image.RGBA, error)) {
	router.GET("/api/preview", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		frame, err := capture(ctx)
		if err != nil {
			replyError(c, err)
			return
		}
		data, err := previewJPEG(frame)
		if err != nil {
			replyError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "image/jpeg", data)
	})
}

func previewJPEG(frame *image.RGBA) ([]byte, error) {
	var source image.Image = frame
	const maxWidth = 640
	if frame.Bounds().Dx() > maxWidth {
		height := max(1, frame.Bounds().Dy()*maxWidth/frame.Bounds().Dx())
		small := image.NewRGBA(image.Rect(0, 0, maxWidth, height))
		xdraw.ApproxBiLinear.Scale(small, small.Bounds(), frame, frame.Bounds(), draw.Src, nil)
		source = small
	}
	var data bytes.Buffer
	err := jpeg.Encode(&data, source, &jpeg.Options{Quality: 85})
	return data.Bytes(), err
}
