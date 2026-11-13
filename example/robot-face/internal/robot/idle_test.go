package robot

import (
	"context"
	"testing"
	"time"

	goui "github.com/lwy183178053/GoUI"
)

func TestRandomBlinkIntervalsStayWithinRangeAndVary(t *testing.T) {
	seen := make(map[time.Duration]bool)
	for range 100 {
		interval := randomBlinkInterval()
		if interval < minBlinkInterval || interval > maxBlinkInterval {
			t.Fatalf("眨眼间隔越界：%s", interval)
		}
		seen[interval] = true
	}
	if len(seen) < 2 {
		t.Fatal("眨眼间隔不应固定")
	}
}

func startIdle(t *testing.T, robot *Controller) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- robot.RunIdle(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("取消后眨眼等待未退出")
		}
	})
	return cancel
}

func TestIdleReplaysNormalWithoutLoadingTheSourceAgain(t *testing.T) {
	robot, _ := testRobot(t)
	old := robot.generation
	// 将下一次眨眼设为已到期；RunIdle 只调用 Play，已加载的 GIF 无需重新读盘。
	robot.blinkAt = time.Now().Add(-time.Second)
	dispatched := make(chan struct{}, 1)
	original := robot.dispatch
	robot.dispatch = func(ctx context.Context, update func(*goui.Page) error) error {
		err := original(ctx, update)
		dispatched <- struct{}{}
		return err
	}
	startIdle(t, robot)
	select {
	case <-dispatched:
	case <-time.After(time.Second):
		t.Fatal("到期后没有重播眨眼")
	}
	robot.mu.RLock()
	defer robot.mu.RUnlock()
	if robot.generation != old+1 || !robot.blinkAt.IsZero() || robot.current.Expression.Name != "normal" {
		t.Fatal("眨眼重播后应等动画完成再安排下一次")
	}
}

func TestQueuedBlinkDoesNotInterruptANewAction(t *testing.T) {
	robot, _ := testRobot(t)
	robot.blinkAt = time.Now().Add(-time.Second)
	queued := make(chan func(*goui.Page) error, 1)
	original := robot.dispatch
	robot.dispatch = func(ctx context.Context, update func(*goui.Page) error) error {
		queued <- update
		<-ctx.Done()
		return ctx.Err()
	}
	cancel := startIdle(t, robot)
	var blink func(*goui.Page) error
	select {
	case blink = <-queued:
	case <-time.After(time.Second):
		t.Fatal("眨眼任务没有提交")
	}
	// 模拟窗口先执行新动作，然后才处理旧的眨眼通知。
	robot.mu.Lock()
	if err := robot.playExpression(expressions[13], true); err != nil {
		robot.mu.Unlock()
		t.Fatal(err)
	}
	robot.mu.Unlock()
	if err := original(context.Background(), blink); err != nil {
		t.Fatal(err)
	}
	if got := robot.Emotion(); got.Expression.Name != "music" || !got.Loop {
		t.Fatalf("旧眨眼通知打断了音乐：%+v", got)
	}
	cancel()
}

func TestIdleCancellationDoesNotWaitForBlinkTimer(t *testing.T) {
	robot, _ := testRobot(t)
	robot.blinkAt = time.Now().Add(time.Hour)
	cancel := startIdle(t, robot)
	cancel()
}
