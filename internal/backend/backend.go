// Package backend 定义平台显示后端需要实现的接口。
package backend

import "image"

// FrameSource 返回只读的预乘 RGBA 帧。调用方必须在再次调用 Frame 前读完画面；
// 实现可能在下次调用时复用帧内存。调用应在同一线程串行进行。
type FrameSource interface {
	Frame(width, height int) (*image.RGBA, error)
}

// Presenter 管理平台窗口和事件循环。窗口关闭时 Run 返回，
// 窗口尺寸变化时会请求对应尺寸的画面。
type Presenter interface {
	Run(FrameSource) error
}

// PointerHandler 接收鼠标移动和左键事件。
type PointerHandler interface {
	PointerMove(x, y int) error
	PointerDown(x, y int) error
	PointerUp(x, y int) error
}

// Updater 在同一主循环中处理排队的业务更新；处理完再接收鼠标和绘制。
type Updater interface {
	Update() error
}
