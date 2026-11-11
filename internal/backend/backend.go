// Package backend 定义平台显示后端需要实现的接口。
package backend

import "image"

// FrameSource 返回只读的预乘 RGBA 帧。画面未变时可以复用同一帧，
// 内容变化时返回新帧；调用应在同一线程串行进行。
type FrameSource interface {
	Frame(width, height int) (*image.RGBA, error)
}

// Presenter 管理平台窗口和事件循环。窗口关闭时 Run 返回，
// 窗口尺寸变化时会请求对应尺寸的画面。
type Presenter interface {
	Run(FrameSource) error
}

// PointerHandler 接收窗口后端转换来的左键按下和释放坐标。
type PointerHandler interface {
	PointerDown(x, y int)
	PointerUp(x, y int) error
}
