package robot

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"example.com/goui-robot-face/internal/conversation"
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
	return startTestVoice(t, controller, microphoneServices{record: record, save: save})
}

func startTestVoice(t *testing.T, controller *Controller, services microphoneServices) context.CancelFunc {
	t.Helper()
	if services.validate == nil {
		services.validate = func() error { return nil }
	}
	if services.save == nil {
		services.save = func([]byte) (string, error) { return "test.wav", nil }
	}
	if services.respond == nil {
		services.respond = func(_ context.Context, _ []byte, emit func(conversation.Update) error) error {
			return emit(conversation.Update{Phase: conversation.Answering, Text: "测试回答"})
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- controller.runMicrophone(ctx, services) }()
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
	waitForMicrophoneText(t, controller, "测试回答")
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
		waitForMicrophoneText(t, controller, "测试回答")
	}
}

func TestMicrophoneSaveErrorDoesNotBlockConversation(t *testing.T) {
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
	waitForMicrophoneText(t, controller, "测试回答")
	controller.micClicks <- struct{}{}
	deadline := time.Now().Add(3 * time.Second)
	for attempts.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	waitForMicrophoneText(t, controller, "测试回答")
	if attempts.Load() != 2 {
		t.Fatal("保存失败后应能再次录音")
	}
}

// 无论还在请求服务还是已经播放，打断都先归还旧任务，再开始下一次录音。
func TestMicrophoneInterruptsResponseBeforeRecordingAgain(t *testing.T) {
	for _, phase := range []conversation.Phase{conversation.Recognizing, conversation.Thinking, conversation.Answering, conversation.Speaking} {
		t.Run(string(phase), func(t *testing.T) {
			controller, _ := testRobot(t)
			var recordings atomic.Int32
			var responseStopped atomic.Bool
			startTestVoice(t, controller, microphoneServices{
				record: func(ctx context.Context, ready func()) ([]byte, error) {
					if recordings.Add(1) > 1 && !responseStopped.Load() {
						t.Error("上一轮还未释放就重新录音")
					}
					ready()
					<-ctx.Done()
					return []byte("recording"), nil
				},
				respond: func(ctx context.Context, _ []byte, emit func(conversation.Update) error) error {
					if err := emit(conversation.Update{Phase: phase, Text: "旧回答处理中"}); err != nil {
						return err
					}
					<-ctx.Done()
					if err := emit(conversation.Update{Phase: phase, Text: "过期字幕"}); err == nil {
						t.Error("取消后不应接受旧任务的更新")
					}
					responseStopped.Store(true)
					return ctx.Err()
				},
			})
			controller.micClicks <- struct{}{}
			waitForMicrophoneText(t, controller, "正在听")
			controller.micClicks <- struct{}{}
			waitForMicrophoneText(t, controller, "旧回答处理中")
			controller.micClicks <- struct{}{}
			waitForMicrophoneText(t, controller, "正在听")
			if recordings.Load() != 2 || !responseStopped.Load() {
				t.Fatal("打断应取消回答并重新倾听")
			}
		})
	}
}

func TestMicrophoneMissingConfigDoesNotStartRecording(t *testing.T) {
	controller, _ := testRobot(t)
	startTestVoice(t, controller, microphoneServices{
		validate: func() error { return errors.New("请配置模型") },
		record: func(context.Context, func()) ([]byte, error) {
			t.Error("配置缺失时不应打开麦克风")
			return nil, nil
		},
	})
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "请配置模型")
}

func TestMicrophoneShutdownCancelsResponse(t *testing.T) {
	controller, _ := testRobot(t)
	stopped := make(chan struct{})
	cancel := startTestVoice(t, controller, microphoneServices{
		record: func(_ context.Context, ready func()) ([]byte, error) {
			ready()
			return []byte("recording"), nil
		},
		respond: func(ctx context.Context, _ []byte, emit func(conversation.Update) error) error {
			if err := emit(conversation.Update{Phase: conversation.Speaking, Text: "正在播放"}); err != nil {
				return err
			}
			<-ctx.Done()
			close(stopped)
			return ctx.Err()
		},
	})
	controller.micClicks <- struct{}{}
	waitForMicrophoneText(t, controller, "正在播放")
	cancel()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("关闭窗口必须取消正在进行的回答")
	}
}
