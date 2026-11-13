package software

import (
	"fmt"
	"time"

	"github.com/lwy183178053/GoUI/internal/layout"
	"github.com/lwy183178053/GoUI/internal/ui"
)

// FinishImage 在主循环中请求分段 GIF 回正，不阻塞调用者。
func (r *Renderer) FinishImage(id string, onDone func() error) error {
	node, _, err := r.node(id)
	if err != nil {
		return err
	}
	player := r.imagePlayers[node]
	if player == nil || player.animation.Segments == nil {
		return fmt.Errorf("UI id %q is not a segmented GIF image", id)
	}
	if onDone == nil {
		return fmt.Errorf("image finish callback is required")
	}
	if !nodeVisible(r.document.Root, node) {
		return fmt.Errorf("UI id %q is hidden", id)
	}
	return player.finish(time.Now(), func() error {
		if err := onDone(); err != nil {
			return &ui.Error{Source: node.Source, Err: fmt.Errorf("finish GIF %q: %w", node.Image.Source, err)}
		}
		return nil
	})
}

func (r *Renderer) advanceImageAnimations(box *layout.Box, now time.Time) error {
	var completed []func() error
	if err := r.advanceImageTree(box, now, &completed); err != nil {
		return err
	}
	// 先走完整棵树再执行回调，避免显隐操作使结果依赖 XML 中的先后顺序。
	for _, callback := range completed {
		if err := callback(); err != nil {
			return err
		}
	}
	if len(completed) > 0 {
		// 回调可能显示另一张 GIF，绘制前先准备其首帧，避免一帧空白或重影。
		// 这一遍不取回调，新完成的播放留到下一帧处理，避免回调递归。
		return r.advanceImageTree(r.layout, now, nil)
	}
	return nil
}

// 沿布局树推进 GIF；父节点隐藏时，不再访问它的子树。
func (r *Renderer) advanceImageTree(box *layout.Box, now time.Time, completed *[]func() error) error {
	if len(r.imagePlayers) == 0 || !box.Node.Visible {
		return nil
	}
	if player := r.imagePlayers[box.Node]; player != nil {
		changed, err := player.advance(now)
		if err != nil {
			return &ui.Error{Source: box.Node.Source, Err: fmt.Errorf("advance GIF %q: %w", box.Node.Image.Source, err)}
		}
		if changed {
			delete(r.images, box.Node)
			delete(r.layers, box.Node)
			r.invalidate(box.Pixels)
		}
		if completed != nil {
			if callback := player.takeCompletion(); callback != nil {
				*completed = append(*completed, callback)
			}
		}
	}
	for _, child := range box.Children {
		if err := r.advanceImageTree(child, now, completed); err != nil {
			return err
		}
	}
	return nil
}

// 显隐状态改变时一起重置子树，重新显示的 GIF 就会从头播放。
func (r *Renderer) resetImageAnimations(node *ui.Node) {
	if len(r.imagePlayers) == 0 {
		return
	}
	if player := r.imagePlayers[node]; player != nil {
		player.reset()
		delete(r.images, node)
		delete(r.layers, node)
	}
	for _, child := range node.Children {
		r.resetImageAnimations(child)
	}
}
