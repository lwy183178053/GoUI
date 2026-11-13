package robot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	goui "github.com/lwy183178053/GoUI"
)

const (
	minBlinkInterval = 2 * time.Second
	maxBlinkInterval = 5 * time.Second
)

func randomBlinkInterval() time.Duration {
	return minBlinkInterval + time.Duration(rand.Int64N(int64(maxBlinkInterval-minBlinkInterval)+1))
}

// wakeEffects 通知等待循环重新读取计划，连续的通知可以合并。
func (c *Controller) wakeEffects() {
	select {
	case c.effectsWake <- struct{}{}:
	default:
	}
}

// RunEffects 共用一个可取消的等待循环，管理随机眨眼和打字机效果。
// 只有计划到期才提交更新；GIF 帧仍由框架原有的窗口帧循环推进。
func (c *Controller) RunEffects(ctx context.Context) error {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		c.mu.RLock()
		next := c.blinkAt
		textAt := c.nextTextTime()
		if !textAt.IsZero() && (next.IsZero() || textAt.Before(next)) {
			next = textAt
		}
		c.mu.RUnlock()

		timer.Stop()
		var ready <-chan time.Time
		if !next.IsZero() {
			timer.Reset(max(0, time.Until(next)))
			ready = timer.C
		}
		select {
		case <-ctx.Done():
			return nil
		case <-c.effectsWake:
			continue
		case <-ready:
			err := c.dispatch(ctx, func(*goui.Page) error {
				c.mu.Lock()
				defer c.mu.Unlock()
				// 计划可能已经被新请求替换，执行时只读取当前状态。
				now := time.Now()
				if err := c.advanceText(now); err != nil {
					return err
				}
				if c.current.Expression.Name == "normal" && !c.blinkAt.IsZero() && !now.Before(c.blinkAt) {
					return c.playExpression(c.current.Expression, false)
				}
				return nil
			})
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("更新机器人显示: %w", err)
			}
		}
	}
}
