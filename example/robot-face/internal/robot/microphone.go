package robot

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"log"

	"example.com/goui-robot-face/internal/conversation"
	"example.com/goui-robot-face/internal/voice"
	goui "github.com/lwy183178053/GoUI"
)

// 状态决定按钮外观和点击行为；网络请求和声音都不在窗口回调中执行。
type microphoneState uint8

const (
	microphoneIdle microphoneState = iota
	microphoneOpening
	microphoneListening
	microphoneStopping
	microphoneProcessing
	microphoneSpeaking
)

var (
	microphoneIdleColor      = color.NRGBA{R: 24, G: 45, B: 61, A: 255}
	microphoneRecordingColor = color.NRGBA{R: 15, G: 137, B: 185, A: 255}
)

// 用同一组入口连接真实服务和测试替身，控制器不关心 HTTP 或设备实现。
type microphoneServices struct {
	validate func() error
	record   func(context.Context, func()) ([]byte, error)
	save     func([]byte) (string, error)
	respond  func(context.Context, []byte, func(conversation.Update) error) error
}

type microphoneUpdate struct {
	state microphoneState
	text  string
}

func (c *Controller) onMicrophoneClick(*goui.Page) error {
	select {
	case c.micClicks <- struct{}{}:
	default:
	}
	return nil
}

// RunMicrophone 管理一次完整对话：开始/结束录音，识别，回答和播放。
// 回答期间再点击，先取消并归还旧任务的设备，再开始下一轮，避免声音重叠。
func (c *Controller) RunMicrophone(ctx context.Context, session *conversation.Session) error {
	err := c.runMicrophone(ctx, microphoneServices{
		validate: session.Validate, record: voice.Record, save: voice.SaveRecording,
		respond: func(ctx context.Context, audio []byte, emit func(conversation.Update) error) error {
			return session.Respond(ctx, audio, emit, voice.Play)
		},
	})
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func (c *Controller) runMicrophone(ctx context.Context, services microphoneServices) error {
	state := microphoneIdle
	var cancelTurn, stopRecording context.CancelFunc
	var updates <-chan microphoneUpdate
	var finished <-chan error
	restart := false
	defer func() {
		if cancelTurn != nil {
			cancelTurn()
			<-finished // 退出前释放设备；旧任务不能更新新一轮的字幕。
		}
	}()

	start := func() error {
		if err := services.validate(); err != nil {
			return c.updateMicrophone(ctx, microphoneIdle, err.Error())
		}
		if err := c.updateMicrophone(ctx, microphoneOpening, "正在打开麦克风…"); err != nil {
			return err
		}
		state = microphoneOpening
		turnCtx, cancel := context.WithCancel(ctx)
		recordingCtx, stop := context.WithCancel(turnCtx)
		cancelTurn, stopRecording = cancel, stop
		progress := make(chan microphoneUpdate)
		done := make(chan error, 1)
		updates, finished = progress, done
		emit := func(update microphoneUpdate) error {
			if err := turnCtx.Err(); err != nil {
				return err
			}
			select {
			case progress <- update:
				return nil
			case <-turnCtx.Done():
				return turnCtx.Err()
			}
		}
		go func() {
			defer stop()
			done <- runVoiceTurn(turnCtx, recordingCtx, services, emit)
		}()
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.micClicks:
			switch state {
			case microphoneOpening, microphoneStopping:
				continue
			case microphoneListening:
				state = microphoneStopping
				stopRecording() // 只结束采集，整轮对话的 context 仍有效。
				if err := c.updateMicrophone(ctx, state, "正在结束录音…"); err != nil {
					return err
				}
			case microphoneProcessing, microphoneSpeaking:
				restart = true
				state = microphoneOpening
				cancelTurn()
				if err := c.updateMicrophone(ctx, state, "已打断，正在打开麦克风…"); err != nil {
					return err
				}
			default:
				if err := start(); err != nil {
					return err
				}
			}
		case update := <-updates:
			if restart {
				continue
			}
			state = update.state
			if err := c.updateMicrophone(ctx, state, update.text); err != nil {
				return err
			}
		case err := <-finished:
			cancelTurn()
			cancelTurn, stopRecording, updates, finished = nil, nil, nil, nil
			if ctx.Err() != nil {
				return nil
			}
			state = microphoneIdle
			if restart {
				restart = false
				if err := start(); err != nil {
					return err
				}
				continue
			}
			message := ""
			if err != nil {
				log.Printf("语音对话失败：%v", err)
				message = "语音对话失败：" + err.Error()
			}
			if err := c.updateMicrophone(ctx, state, message); err != nil {
				return err
			}
		}
	}
}

// runVoiceTurn 只在后台运行，状态统一送回上面的循环，避免过期请求覆盖字幕。
func runVoiceTurn(ctx, recordingCtx context.Context, services microphoneServices, emit func(microphoneUpdate) error) error {
	audio, err := services.record(recordingCtx, func() {
		_ = emit(microphoneUpdate{microphoneListening, "正在听，请说话；再次点击麦克风结束"})
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return errors.New("本次未录到声音，请再次点击麦克风")
		}
		return fmt.Errorf("录音: %w", err)
	}
	// 最近一次录音仅用于排查；缓存不可写不影响本次识别和对话。
	if filename, err := services.save(audio); err != nil {
		log.Printf("保存录音失败：%v", err)
	} else {
		log.Printf("录音已保存：%s", filename)
	}
	return services.respond(ctx, audio, func(update conversation.Update) error {
		state := microphoneProcessing
		if update.Phase == conversation.Speaking {
			state = microphoneSpeaking
		}
		return emit(microphoneUpdate{state, update.Text})
	})
}

// 空 message 只更新按钮；回答结束后保留最后一段文字。
func (c *Controller) updateMicrophone(ctx context.Context, state microphoneState, message string) error {
	return c.dispatch(ctx, func(*goui.Page) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		fill := microphoneIdleColor
		if state != microphoneIdle {
			fill = microphoneRecordingColor
		}
		if err := c.microphone.SetColor(fill); err != nil {
			return err
		}
		if err := c.microphone.SetDisabled(state == microphoneOpening || state == microphoneStopping); err != nil {
			return err
		}
		if message != "" {
			return c.showText(message, false)
		}
		return nil
	})
}
