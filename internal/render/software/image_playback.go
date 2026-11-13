package software

import (
	"errors"
	"image"
	"time"

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
	exit         *imageExit
	mode         imagePlayMode
	completion   *imageCompletion
}

// imageExit 记录请求回正时的播放位置，之后沿原 GIF 的时间轴走到末尾。
type imageExit struct {
	elapsed  time.Duration
	position time.Duration
}

type imagePlayMode uint8

const (
	imageAuto imagePlayMode = iota // 遵守 XML 与 GIF 文件本身的设置。
	imageOnce
	imageLoop
)

// imageCompletion 同时用于完整播放与分段回正，重播或隐藏后旧回调失效。
type imageCompletion struct {
	onDone   func() error
	complete bool
}

func newImagePlayback(animation *ui.GIFAnimation) *imagePlayback {
	return &imagePlayback{animation: animation, current: -1, duration: animation.Duration()}
}

func (p *imagePlayback) advance(now time.Time) (bool, error) {
	if p.started.IsZero() {
		p.started = now
	}
	if p.decoder == nil {
		p.decoder = p.animation.NewPlayer()
	}
	elapsed := now.Sub(p.started)
	frame := p.frameAt(elapsed)
	if p.completion != nil && p.positionAt(elapsed) >= p.duration {
		p.completion.complete = true
	}
	if frame == p.current {
		return false, nil
	}
	pixels, err := p.decoder.Frame(frame)
	if err != nil {
		return false, err
	}
	p.currentFrame = pixels
	p.current = frame
	return true, nil
}

func (p *imagePlayback) frameAt(elapsed time.Duration) int {
	position := p.positionAt(elapsed)
	for i, frame := range p.animation.Frames {
		if position < frame.Delay {
			return i
		}
		position -= frame.Delay
	}
	return len(p.animation.Frames) - 1
}

// positionAt 将实际经过时间映射到 GIF 原时间轴；帧速率始终来自 GIF 本身。
func (p *imagePlayback) positionAt(elapsed time.Duration) time.Duration {
	elapsed = max(0, elapsed)
	if p.duration <= 0 {
		return 0
	}
	switch p.mode {
	case imageOnce:
		return min(elapsed, p.duration)
	case imageLoop:
		return elapsed % p.duration
	}
	segments := p.animation.Segments
	if p.exit != nil {
		position := p.exit.position + max(0, elapsed-p.exit.elapsed)
		// 在进入段就收到切换请求时，播完进入段后跳过保持/循环段。
		if segments != nil && p.exit.position < segments.Start && position >= segments.Start {
			position += segments.End - segments.Start
		}
		return min(position, p.duration)
	}
	if segments != nil {
		if elapsed < segments.Start {
			return elapsed
		}
		if segments.Start == segments.End {
			return segments.Start
		}
		return segments.Start + (elapsed-segments.Start)%(segments.End-segments.Start)
	}
	// 普通 GIF 保留原有循环规则；文件名里即使带参数也不会自动启用分段。
	loops := p.animation.LoopCount
	if loops < 0 && elapsed >= p.duration {
		return p.duration
	}
	if loops > 0 && elapsed/p.duration > time.Duration(loops) {
		return p.duration
	}
	return elapsed % p.duration
}

func (p *imagePlayback) finish(now time.Time, onDone func() error) error {
	if p.exit != nil {
		return errors.New("GIF finish has already been requested")
	}
	if p.started.IsZero() {
		p.started = now
	}
	elapsed := max(0, now.Sub(p.started))
	position := p.positionAt(elapsed)
	p.mode = imageAuto // 请求结束后沿当前时间轴走到末尾，不重播进入段。
	p.exit = &imageExit{elapsed: elapsed, position: position}
	p.completion = &imageCompletion{onDone: onDone}
	return nil
}

// play 覆盖文件的循环次数；同一资源重播只重置逐帧解码状态。
func (p *imagePlayback) play(loop bool, onDone func() error) {
	p.reset()
	p.mode = imageOnce
	if loop {
		p.mode = imageLoop
	} else if onDone != nil {
		p.completion = &imageCompletion{onDone: onDone}
	}
}

func (p *imagePlayback) takeCompletion() func() error {
	completion := p.completion
	if completion == nil || !completion.complete || completion.onDone == nil {
		return nil
	}
	callback := completion.onDone
	completion.onDone = nil // 每次播放只通知一次，末帧可以继续显示。
	return func() error {
		// 同帧其它回调可能重播、换源或隐藏此图片。
		if p.completion != completion {
			return nil
		}
		return callback()
	}
}

// reset 让下一次显示从 GIF 第一帧开始，释放画布并取消待执行的完成回调。
func (p *imagePlayback) reset() {
	p.decoder = nil
	p.currentFrame = nil
	p.started = time.Time{}
	p.current = -1
	p.exit = nil
	p.mode = imageAuto
	p.completion = nil
}

func (p *imagePlayback) frame() image.Image {
	return p.currentFrame
}
