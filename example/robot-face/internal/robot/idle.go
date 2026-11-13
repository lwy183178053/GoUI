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

// wakeIdle 只通知等待循环重新读取计划，连续的通知可以合并。
func (c *Controller) wakeIdle() {
	select {
	case c.idleWake <- struct{}{}:
	default:
	}
}

// RunIdle 管理正常眨眼的随机间隔，由 main 启动一次并随窗口关闭取消。
// 等待不占用绘制线程；GIF 帧仍由框架原有的窗口帧循环推进。
func (c *Controller) RunIdle(ctx context.Context) error {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		c.mu.RLock()
		next := c.blinkAt
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
		case <-c.idleWake:
			continue
		case <-ready:
			err := c.dispatch(ctx, func(*goui.Page) error {
				c.mu.Lock()
				defer c.mu.Unlock()
				// 定时通知可能已经排队；在主循环再次检查，避免打断刚切入的动作。
				if c.current.Expression.Name != "normal" || c.blinkAt.IsZero() || time.Now().Before(c.blinkAt) {
					return nil
				}
				return c.playExpression(c.current.Expression, false)
			})
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("播放正常眨眼: %w", err)
			}
		}
	}
}
