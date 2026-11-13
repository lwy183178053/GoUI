package goui

import "errors"

// Image 显示静态图片或 GIF，继承 Block 的样式与动画方法。
type Image struct{ Block }

// SetSource 从 UI 目录加载并替换当前图片，保留布局、样式和子节点。
// 成功后从首帧播放，原 GIF 与待完成回调一并释放；失败时保留原图片。
// 同一路径也会重新加载；外部 goroutine 通过 Application.Dispatch 调用。
func (i *Image) SetSource(src string) error {
	if i == nil {
		return errors.New("image is not connected to an open application")
	}
	target, err := i.target()
	if err != nil {
		return err
	}
	return target.SetImageSource(i.id, src)
}

// Play 从首帧重播已加载的 auto GIF，不重复读取文件。
// loop=false 完整播放一次，末帧显示后在主循环调用可选的 onDone。
// loop=true 持续循环，此时 onDone 必须为 nil；GIF 自带的循环次数会被覆盖。
// 重播、换源或隐藏节点会取消旧回调。回调内直接操作组件，不要再次 Dispatch。
func (i *Image) Play(loop bool, onDone func(*Page) error) error {
	if i == nil {
		return errors.New("image is not connected to an open application")
	}
	target, err := i.target()
	if err != nil {
		return err
	}
	var callback func() error
	if onDone != nil {
		callback = func() error { return onDone(i.page) }
	}
	return target.PlayImage(i.id, loop, callback)
}

// Finish 让当前 GIF 完成本轮；循环中的普通 GIF 不会从头重播。
// 分段 GIF 则播完进入段或当前循环，再播放回正段。
// 仅适用于当前可见的 GIF，每次播放只能请求一次。
// 完成后在窗口主循环中调用 onDone；回调可以直接操作组件，不要再次 Dispatch。
// 隐藏图片或其父节点会取消此次完成回调，重新显示则从头播放。
func (i *Image) Finish(onDone func(*Page) error) error {
	if i == nil {
		return errors.New("image is not connected to an open application")
	}
	if onDone == nil {
		return errors.New("image finish callback is required")
	}
	target, err := i.target()
	if err != nil {
		return err
	}
	return target.FinishImage(i.id, func() error { return onDone(i.page) })
}
