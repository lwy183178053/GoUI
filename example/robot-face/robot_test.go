package main

import (
	"context"
	"sync"
	"testing"
)

func TestRobotWaitsForReturnAndKeepsSameGIFPlaying(t *testing.T) {
	robot, _ := testRobot(t)
	got, err := robot.set(context.Background(), vector{9, 9})
	if err != nil || got.Expression.Name != "normal" || got.Target.Name != "happy" || !got.Transitioning {
		t.Fatalf("应先回正，目标为开心：%+v, %v", got, err)
	}
	got, err = robot.set(context.Background(), vector{10, 10})
	if err != nil || got.Vector != (vector{10, 10}) || got.Target.Name != "happy" {
		t.Fatalf("等待回正时不应重复 Finish：%+v, %v", got, err)
	}
	// 手动调用完成回调模拟播放器通知，单元测试不依赖真实窗口或等待时间。
	if err := robot.showTarget(nil); err != nil {
		t.Fatal(err)
	}
	got = robot.state()
	if got.Expression.Name != "happy" || got.Transitioning {
		t.Fatalf("回正完成后应显示开心：%+v", got)
	}
	// 同一张 GIF 的新坐标不应调用显隐方法，也不应请求它退出。
	robot.images["happy"] = nil
	got, err = robot.set(context.Background(), vector{9, 10})
	if err != nil || got.Expression.Name != "happy" || got.Transitioning {
		t.Fatalf("重复选中应继续播放：%+v, %v", got, err)
	}
}

func TestRobotKeepsOnlyLatestTargetDuringReturn(t *testing.T) {
	robot, _ := testRobot(t)
	for _, input := range []vector{{9, 9}, {1, 2}, {1, 9}} {
		if _, err := robot.set(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	got := robot.state()
	if got.Expression.Name != "normal" || got.Target.Name != "angry" || !got.Transitioning {
		t.Fatalf("回正期间应保留当前和最新目标：%+v", got)
	}
	// 中间目标不需要显示或播放。
	robot.images["happy"], robot.images["sad"] = nil, nil
	if err := robot.showTarget(nil); err != nil {
		t.Fatal(err)
	}
	got = robot.state()
	if got.Expression.Name != "angry" || got.Target.Name != "angry" || got.Transitioning {
		t.Fatalf("应直接切到最新目标：%+v", got)
	}
	if err := robot.showTarget(nil); err != nil || robot.state() != got {
		t.Fatal("重复完成通知不应重新切换")
	}
}

func TestRobotCanReturnToSameExpressionAndRestart(t *testing.T) {
	robot, _ := testRobot(t)
	for _, input := range []vector{{9, 9}, {5, 5}} {
		if _, err := robot.set(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	if err := robot.showTarget(nil); err != nil {
		t.Fatal(err)
	}
	got := robot.state()
	if got.Expression.Name != "normal" || got.Transitioning {
		t.Fatalf("目标改回当前图时，应完成回正再重新播放：%+v", got)
	}
	if _, err := robot.set(context.Background(), vector{1, 9}); err != nil {
		t.Fatalf("重新播放后应允许再次 Finish：%v", err)
	}
}

func TestRobotRejectsInvalidInputWithoutChangingState(t *testing.T) {
	robot, _ := testRobot(t)
	before := robot.state()
	if _, err := robot.set(context.Background(), vector{-1, 5}); err == nil || robot.state() != before {
		t.Fatal("非法向量应报错且保留原状态")
	}
	if err := robot.images["normal"].SetVisible(false); err != nil {
		t.Fatal(err)
	}
	if _, err := robot.set(context.Background(), vector{9, 9}); err == nil || robot.state() != before {
		t.Fatal("请求回正失败时不应修改状态")
	}
}

func TestRobotHandlesConcurrentStateRequests(t *testing.T) {
	robot, _ := testRobot(t)
	var workers sync.WaitGroup
	failures := make(chan error, len(expressions))
	for _, item := range expressions {
		workers.Go(func() {
			_, err := robot.set(context.Background(), vector{item.Valence, item.Arousal})
			failures <- err
			_ = robot.state()
		})
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := robot.state()
	nearest, err := nearestExpression(got.Vector)
	if err != nil || nearest.Name != got.Target.Name || got.Expression.Name != "normal" {
		t.Fatalf("并发更新后的目标不一致：%+v, %v", got, err)
	}
	if err := robot.showTarget(nil); err != nil {
		t.Fatal(err)
	}
	if got := robot.state(); got.Expression != got.Target || got.Transitioning {
		t.Fatalf("回正后实际表情应与目标一致：%+v", got)
	}
}
