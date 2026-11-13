package software

import (
	"errors"
	"image"
)

// Snapshot 只复制最近绘制的帧，不重新布局或推进动画。
// 与其它渲染操作一样，必须在窗口主循环中串行调用。
func (r *Renderer) Snapshot() (*image.RGBA, error) {
	if r.closed {
		return nil, errors.New("renderer is closed")
	}
	if r.published == nil {
		return nil, errors.New("no frame has been rendered yet")
	}
	frame := image.NewRGBA(r.published.Bounds())
	copy(frame.Pix, r.published.Pix)
	return frame, nil
}
