package software

import (
	"fmt"
	"image"
	"time"

	"github.com/lwy183178053/GoUI/internal/layout"
	"github.com/lwy183178053/GoUI/internal/ui"
)

// imagePlayback 只为正在显示的 GIF 合成帧，不单独启动定时器。
type imagePlayback struct {
	animation    *ui.GIFAnimation
	decoder      *ui.GIFFramePlayer
	currentFrame image.Image
	duration     time.Duration
	started      time.Time
	current      int
}

func newImagePlayback(animation *ui.GIFAnimation) *imagePlayback {
	player := &imagePlayback{animation: animation, current: -1}
	for _, frame := range animation.Frames {
		player.duration += frame.Delay
	}
	return player
}

func (p *imagePlayback) advance(now time.Time) (bool, error) {
	if p.started.IsZero() {
		p.started = now
		p.decoder = p.animation.NewPlayer()
	}
	frame := p.frameAt(now.Sub(p.started))
	if frame == p.current {
		return false, nil
	}
	image, err := p.decoder.Frame(frame)
	if err != nil {
		return false, err
	}
	p.currentFrame = image
	p.current = frame
	return true, nil
}

func (p *imagePlayback) frameAt(elapsed time.Duration) int {
	if len(p.animation.Frames) < 2 || p.duration <= 0 || elapsed <= 0 {
		return 0
	}
	if p.animation.LoopCount < 0 && elapsed >= p.duration {
		return len(p.animation.Frames) - 1
	}
	if p.animation.LoopCount > 0 && elapsed >= p.duration*time.Duration(p.animation.LoopCount+1) {
		return len(p.animation.Frames) - 1
	}
	elapsed %= p.duration
	for i, frame := range p.animation.Frames {
		if elapsed < frame.Delay {
			return i
		}
		elapsed -= frame.Delay
	}
	return len(p.animation.Frames) - 1
}

// reset 让下一次显示从 GIF 第一帧开始，也释放当前帧画布。
func (p *imagePlayback) reset() {
	p.decoder = nil
	p.currentFrame = nil
	p.started = time.Time{}
	p.current = -1
}

func (p *imagePlayback) frame() image.Image {
	return p.currentFrame
}

// 沿布局树推进 GIF；父节点隐藏时，不再访问它的子树。
func (r *Renderer) advanceImageAnimations(box *layout.Box, now time.Time) error {
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
	}
	for _, child := range box.Children {
		if err := r.advanceImageAnimations(child, now); err != nil {
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
