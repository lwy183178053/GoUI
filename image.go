package goui

import "errors"

// Image 显示静态图片或 GIF，继承 Block 的样式与动画方法。
type Image struct{ Block }

// Finish 让分段 GIF 播完进入段或当前循环，再播放回正段。
// 仅适用于当前可见且设置 playback="segmented" 的 GIF，每次播放只能请求一次。
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
