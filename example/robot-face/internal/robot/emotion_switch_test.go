package robot

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func play(t *testing.T, robot *Controller, name string, loop, interrupt bool) EmotionState {
	t.Helper()
	state, err := robot.SetExpression(context.Background(), ExpressionRequest{Name: name, Loop: loop, Interrupt: interrupt})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestRobotSwitchesImmediatelyAndReturnsToNormal(t *testing.T) {
	robot, _ := testRobot(t)
	got := play(t, robot, "happy", false, true)
	if got.Expression.Name != "happy" || got.Loop || got.Pending != nil {
		t.Fatalf("应立即播放一次开心：%+v", got)
	}
	// 模拟框架在完整 GIF 播完后通知，不等待真实素材时长。
	if err := robot.completeExpression(robot.generation); err != nil {
		t.Fatal(err)
	}
	got = robot.Emotion()
	if got.Expression.Name != "normal" || got.Loop || got.Pending != nil {
		t.Fatalf("动作结束应自动回正常：%+v", got)
	}
	if !robot.blinkAt.IsZero() {
		t.Fatal("正常眨眼尚未完成，不应提前安排下一次")
	}
	before := time.Now()
	if err := robot.completeExpression(robot.generation); err != nil {
		t.Fatal(err)
	}
	if robot.blinkAt.Before(before.Add(minBlinkInterval)) || robot.blinkAt.After(time.Now().Add(maxBlinkInterval)) {
		t.Fatalf("眨眼等待应为 2～5 秒：%v", robot.blinkAt)
	}
}

func TestRobotLoopIsInterruptedAndOldCompletionIsIgnored(t *testing.T) {
	robot, _ := testRobot(t)
	play(t, robot, "happy", false, true)
	old := robot.generation
	got := play(t, robot, "music", true, true)
	if got.Expression.Name != "music" || !got.Loop || !robot.blinkAt.IsZero() {
		t.Fatalf("音乐应立即循环，取消眨眼计划：%+v", got)
	}
	if err := robot.completeExpression(old); err != nil || robot.Emotion() != got {
		t.Fatal("被打断的动作不能把音乐切回正常")
	}
	got = play(t, robot, "badminton", true, true)
	if got.Expression.Name != "badminton" || !got.Loop {
		t.Fatalf("循环中也应立即切换运动：%+v", got)
	}
}

func TestRobotFinishesCurrentRoundAndKeepsOnlyLatestRequest(t *testing.T) {
	robot, _ := testRobot(t)
	play(t, robot, "music", true, true)
	generation := robot.generation
	for _, name := range []string{"happy", "sad", "angry"} {
		got := play(t, robot, name, false, false)
		if got.Expression.Name != "music" || got.Loop || got.Pending == nil || got.Pending.Name != name {
			t.Fatalf("等待本轮时应只更新最新请求：%+v", got)
		}
		if robot.generation != generation {
			t.Fatal("等待切换不能重播当前 GIF")
		}
	}
	if err := robot.completeExpression(generation); err != nil {
		t.Fatal(err)
	}
	if got := robot.Emotion(); got.Expression.Name != "angry" || got.Loop || got.Pending != nil {
		t.Fatalf("本轮结束后应播放最新目标：%+v", got)
	}
	if err := robot.completeExpression(robot.generation); err != nil {
		t.Fatal(err)
	}
	if robot.Emotion().Expression.Name != "normal" {
		t.Fatal("下一动作完成后仍应自动回正常")
	}
}

func TestImmediateRequestCancelsPendingAndCanStartAnotherDeferredSwitch(t *testing.T) {
	robot, _ := testRobot(t)
	play(t, robot, "music", true, true)
	play(t, robot, "sad", false, false)
	old := robot.generation
	got := play(t, robot, "happy", true, true)
	if got.Pending != nil || got.Expression.Name != "happy" || !got.Loop {
		t.Fatalf("立即请求应清除待切换动作：%+v", got)
	}
	if err := robot.completeExpression(old); err != nil || robot.Emotion() != got {
		t.Fatal("已取消的完成通知不应执行待切换动作")
	}
	got = play(t, robot, "music", true, false)
	// 返回状态和只读状态都是副本，不能泄露内部待切换请求。
	got.Pending.Name = "unknown"
	robot.Emotion().Pending.Name = "also-unknown"
	if robot.Emotion().Pending.Name != "music" {
		t.Fatal("待切换请求被外部修改")
	}
	if err := robot.completeExpression(robot.generation); err != nil {
		t.Fatal(err)
	}
	if got := robot.Emotion(); got.Expression.Name != "music" || !got.Loop || got.Pending != nil {
		t.Fatalf("延后切换也应保留目标的循环设置：%+v", got)
	}
}

func TestRobotSameActionRestartsAndNormalAlwaysBlinks(t *testing.T) {
	robot, _ := testRobot(t)
	for _, loop := range []bool{false, false, true, false} {
		previous := robot.generation
		got := play(t, robot, "music", loop, true)
		if got.Loop != loop || robot.generation != previous+1 {
			t.Fatalf("同一动作应重播并应用新模式：%+v", got)
		}
	}
	got := play(t, robot, "normal", true, true)
	if got.Expression.Name != "normal" || got.Loop {
		t.Fatalf("正常表情不连续循环眨眼：%+v", got)
	}
	if err := robot.completeExpression(robot.generation); err != nil {
		t.Fatal(err)
	}
	got = play(t, robot, "music", true, false)
	if got.Expression.Name != "music" || got.Pending != nil {
		t.Fatalf("正常等待眨眼时可以直接开始动作：%+v", got)
	}
}

func TestRobotRejectsInvalidInputWithoutChangingState(t *testing.T) {
	robot, directory := testRobot(t)
	before, generation := robot.Emotion(), robot.generation
	if _, err := robot.SetExpression(context.Background(), ExpressionRequest{Name: "missing", Interrupt: true}); err == nil || robot.Emotion() != before {
		t.Fatal("未知名字应报错且保留原状态")
	}
	if err := os.Remove(filepath.Join(directory, "gif", expressions[1].File)); err != nil {
		t.Fatal(err)
	}
	if _, err := robot.SetExpression(context.Background(), ExpressionRequest{Name: "happy", Interrupt: true}); err == nil || robot.Emotion() != before || robot.generation != generation {
		t.Fatal("新素材缺失时应保留当前表情与播放计划")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := robot.SetExpression(ctx, ExpressionRequest{Name: "music", Interrupt: true}); err == nil || robot.Emotion() != before {
		t.Fatal("取消的请求不应更改表情")
	}
	if err := robot.face.SetVisible(false); err != nil {
		t.Fatal(err)
	}
	if _, err := robot.SetExpression(context.Background(), ExpressionRequest{Name: "music"}); err == nil || robot.Emotion() != before {
		t.Fatal("隐藏的播放器不能接受延后切换，也不能更改状态")
	}
}

func TestRobotHandlesConcurrentStateRequests(t *testing.T) {
	robot, _ := testRobot(t)
	var workers sync.WaitGroup
	for _, item := range expressions {
		workers.Go(func() {
			if _, err := robot.SetExpression(context.Background(), ExpressionRequest{Name: item.Name, Loop: true, Interrupt: true}); err != nil {
				t.Error(err)
			}
			_ = robot.Emotion()
		})
	}
	workers.Wait()
	got := robot.Emotion()
	registered, err := findExpression(got.Expression.Name)
	if err != nil || registered != got.Expression || got.Pending != nil {
		t.Fatalf("并发更新后的状态不一致：%+v, %v", got, err)
	}
}
