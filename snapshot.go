package goui

import (
	"context"
	"image"
)

// Snapshot 复制窗口最近绘制完成的画面，可从 HTTP 等外部 goroutine 调用。
// 它通过 Dispatch 在主循环中复制，不推进动画；返回的像素可由调用方独立使用。
// 不要在组件或 Dispatch 回调内再次调用。窗口尚未绘制或已经关闭时返回错误。
func (app *Application) Snapshot(ctx context.Context) (*image.RGBA, error) {
	var frame *image.RGBA
	err := app.Dispatch(ctx, func(*Page) error {
		var err error
		frame, err = app.renderer.Snapshot()
		return err
	})
	if err != nil {
		return nil, err
	}
	return frame, nil
}
