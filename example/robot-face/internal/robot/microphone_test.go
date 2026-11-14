package robot

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func waitForMicrophoneText(t *testing.T, controller *Controller, text string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state := controller.Display()
		if strings.Contains(state.Text, text) && state.Text == state.VisibleText && !state.Typing {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("未等到麦克风状态 %q，实际 %+v", text, controller.Display())
}

func startTestMicrophone(t *testing.T, controller *Controller,
	record func(context.Context, func()) ([]byte, error), save func([]byte) (string, error),
) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- controller.runMicrophone(ctx, record, save) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("关闭时未释放麦克风任务")
		}
	})
	return cancel
}

func TestMicrophoneClickStartsAndStopsWithoutBlockingDisplay(t *testing.T) {
	controller, _ := testRobot(t)
	if _, err := controller.SetDisplay(context.Background(), "正在显示的旧回答"); err != nil {
		t.Fatal(err)
	}
	var recordings, saved atomic.Int32
	startTestMicrophone(t, controller, func(ctx context.Context, ready func()) ([]byte, error) {
		recordings.Add(1)
		ready()
		<-ctx.Done()
		return []byte("recording"), nil
	}, func(audio []byte) (string, error) {
		if string(audio) != "recording" {
			return "", errors.New("录音内容被修改")
		}
		saved.Add(1)
		return "test.wav", nil
	})

	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "正在听")
	advanceCaption(t, controller, time.Hour)
	if controller.Display().Typing || strings.Contains(controller.Display().VisibleText, "旧回答") {
		t.Fatal("开始倾听时必须清除旧回答的打字任务")
	}
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "录音已结束")
	if recordings.Load() != 1 || saved.Load() != 1 {
		t.Fatal("两次点击应只产生并保存一次录音")
	}
	// 保存完成后可以开始下一次，不会复用已经取消的录音上下文。
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "正在听")
	if recordings.Load() != 2 {
		t.Fatal("应能再次录音")
	}
}

func TestMicrophoneDeviceErrorAllowsRetry(t *testing.T) {
	controller, _ := testRobot(t)
	var attempts atomic.Int32
	startTestMicrophone(t, controller, func(ctx context.Context, ready func()) ([]byte, error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("麦克风被占用")
		}
		ready()
		<-ctx.Done()
		return nil, ctx.Err()
	}, func([]byte) (string, error) { t.Error("失败的录音不应保存"); return "", nil })
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "麦克风被占用")
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "正在听")
}

func TestMicrophoneShutdownCancelsCaptureWithoutSaving(t *testing.T) {
	controller, _ := testRobot(t)
	stopped := make(chan struct{})
	var saved atomic.Bool
	cancel := startTestMicrophone(t, controller, func(ctx context.Context, ready func()) ([]byte, error) {
		ready()
		<-ctx.Done()
		close(stopped)
		return []byte("unfinished"), nil
	}, func([]byte) (string, error) { saved.Store(true); return "unused.wav", nil })
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "正在听")
	cancel()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("窗口关闭必须停止录音")
	}
	if saved.Load() {
		t.Fatal("退出时不能保存未提交的录音")
	}
}

func TestMicrophoneFinishesWithoutAnotherClick(t *testing.T) {
	controller, _ := testRobot(t)
	var saved atomic.Int32
	startTestMicrophone(t, controller, func(ctx context.Context, ready func()) ([]byte, error) {
		ready()
		// 模拟达到录音上限后自行结束，不等待第二次点击。
		return []byte("completed"), nil
	}, func(audio []byte) (string, error) {
		if string(audio) != "completed" {
			return "", errors.New("录音内容被修改")
		}
		saved.Add(1)
		return "test.wav", nil
	})
	for expected := int32(1); expected <= 2; expected++ {
		controller.micClicks <- struct{}{}
		deadline := time.Now().Add(3 * time.Second)
		for saved.Load() < expected && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if saved.Load() != expected {
			t.Fatal("自动结束后应保存一次，并允许再次录音")
		}
		waitForMicrophoneText(t, controller, "录音已结束")
	}
}

func TestMicrophoneSaveErrorAllowsRetry(t *testing.T) {
	controller, _ := testRobot(t)
	var attempts atomic.Int32
	startTestMicrophone(t, controller, func(ctx context.Context, ready func()) ([]byte, error) {
		ready()
		return []byte("completed"), nil
	}, func([]byte) (string, error) {
		if attempts.Add(1) == 1 {
			return "", errors.New("缓存目录不可写")
		}
		return "test.wav", nil
	})
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "缓存目录不可写")
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "录音已结束")
	if attempts.Load() != 2 {
		t.Fatal("保存失败后应能再次录音")
	}
}
