package goui

import (
	"context"
	"errors"

	"github.com/lwy183178053/GoUI/internal/render/software"
)

// pageUpdate 保存外部 goroutine 提交的一次页面更新。
type pageUpdate struct {
	context context.Context
	apply   func(*Page) error
	result  chan error
}

// Dispatch 将页面更新交给窗口主循环执行，并等待执行结果。
// HTTP、网络和设备回调等外部 goroutine 应使用此方法修改组件。
// 初始化和按钮回调已经在主循环中，直接调用组件方法即可，不要调用 Dispatch。
func (app *Application) Dispatch(ctx context.Context, update func(*Page) error) error {
	if ctx == nil {
		return errors.New("dispatch context is required")
	}
	if update == nil {
		return errors.New("page update callback is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if app == nil || !app.running.Load() {
		return errors.New("application is not running")
	}
	request := pageUpdate{context: ctx, apply: update, result: make(chan error, 1)}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-app.done:
		return errors.New("application has stopped")
	case app.updates <- request:
	}
	select {
	case err := <-request.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-app.done:
		return errors.New("application has stopped")
	}
}

// applicationSource 保留 Renderer 的绘制和鼠标接口，只增加排队更新的处理。
type applicationSource struct {
	*software.Renderer
	page    *Page
	updates <-chan pageUpdate
}

func (source *applicationSource) Update() error {
	// 每轮只处理已经排队的任务，持续到来的请求留到下一轮，避免阻塞绘制。
	for remaining := len(source.updates); remaining > 0; remaining-- {
		request := <-source.updates
		err := request.context.Err()
		if err == nil {
			err = request.apply(source.page)
		}
		// 结果通道带缓冲；调用方超时退出也不会卡住窗口。
		request.result <- err
	}
	return nil
}
