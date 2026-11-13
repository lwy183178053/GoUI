package robot

import (
	"context"
	"time"

	goui "github.com/lwy183178053/GoUI"
)

// SetExpression 按名称切换，loop=false 播完一次回正常。
// interrupt=false 时完成当前这一轮再切换；等待期间只保留最新请求。
func (c *Controller) SetExpression(ctx context.Context, request ExpressionRequest) (EmotionState, error) {
	next, err := findExpression(request.Name)
	if err != nil {
		return EmotionState{}, err
	}
	request.Loop = request.Loop && next.Name != "normal"
	var result EmotionState
	err = c.dispatch(ctx, func(*goui.Page) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		// 正常表情在等待下次眨眼时没有动作在播放，可以直接切换。
		if !request.Interrupt && c.blinkAt.IsZero() {
			if c.current.Pending == nil {
				generation := c.generation
				if err := c.face.Finish(func(*goui.Page) error {
					return c.completeExpression(generation)
				}); err != nil {
					return err
				}
			}
			c.current.Pending = &request
			c.current.Loop = false // 当前动作只再完成这一轮。
		} else if err := c.playExpression(next, request.Loop); err != nil {
			return err
		}
		result = c.emotionSnapshot()
		return nil
	})
	return result, err
}

// playExpression 只在主循环且持有 mu 时调用。换源保留 Image 内的文字节点。
func (c *Controller) playExpression(next Expression, loop bool) error {
	if next.File != c.current.Expression.File {
		if err := c.face.SetSource("gif/" + next.File); err != nil {
			return err
		}
	}
	loop = loop && next.Name != "normal"
	generation := c.generation + 1
	var onDone func(*goui.Page) error
	if !loop {
		onDone = func(*goui.Page) error { return c.completeExpression(generation) }
	}
	if err := c.face.Play(loop, onDone); err != nil {
		return err
	}
	c.generation = generation
	c.current = EmotionState{Expression: next, Loop: loop}
	c.blinkAt = time.Time{}
	c.wakeIdle()
	return nil
}

// completeExpression 在真正播完 GIF 后调用，不按文件名猜测动作时长。
func (c *Controller) completeExpression(generation uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if generation != c.generation {
		return nil // 已被新动作打断，旧通知不能覆盖当前状态。
	}
	if pending := c.current.Pending; pending != nil {
		next, err := findExpression(pending.Name)
		if err != nil {
			return err
		}
		return c.playExpression(next, pending.Loop)
	}
	if c.current.Expression.Name == "normal" {
		c.blinkAt = time.Now().Add(randomBlinkInterval())
		c.wakeIdle()
		return nil
	}
	return c.playExpression(expressions[0], false)
}
