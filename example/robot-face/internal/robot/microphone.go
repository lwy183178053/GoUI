package robot

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"log"

	"example.com/goui-robot-face/internal/voice"
	goui "github.com/lwy183178053/GoUI"
)

// 四个状态同时决定按钮外观和是否允许点击，避免多个布尔值组合出矛盾状态。
type microphoneState uint8

const (
	microphoneIdle microphoneState = iota
	microphoneOpening
	microphoneListening
	microphoneStopping
)

var (
	microphoneIdleColor      = color.NRGBA{R: 24, G: 45, B: 61, A: 255}
	microphoneRecordingColor = color.NRGBA{R: 15, G: 137, B: 185, A: 255}
)

type recordingResult struct {
	audio []byte
	err   error
}

// onMicrophoneClick 只发通知，设备操作和文件保存都由后台任务完成。
func (c *Controller) onMicrophoneClick(*goui.Page) error {
	select {
	case c.micClicks <- struct{}{}:
	default:
	}
	return nil
}

// RunMicrophone 管理点击开始、再次点击结束和窗口关闭后的设备释放。
// 当前只保存最近一段 WAV，语音服务接入后在这里提交识别。
func (c *Controller) RunMicrophone(ctx context.Context) error {
	err := c.runMicrophone(ctx, voice.Record, voice.SaveRecording)
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func (c *Controller) runMicrophone(ctx context.Context,
	record func(context.Context, func()) ([]byte, error),
	save func([]byte) (string, error),
) error {
	var cancel context.CancelFunc
	var started <-chan struct{}
	var finished <-chan recordingResult
	state := microphoneIdle
	defer func() {
		if cancel != nil {
			cancel()
			<-finished // 退出前等录音任务归还设备，不保存被窗口关闭打断的片段。
		}
	}()

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
				started = nil
				cancel()
				if err := c.updateMicrophone(ctx, state, "正在结束录音…"); err != nil {
					return err
				}
				continue
			}
			// 先替换旧回答的文字，设备返回音频后才显示“正在听”。
			state = microphoneOpening
			if err := c.updateMicrophone(ctx, state, "正在打开麦克风…"); err != nil {
				return err
			}
			recordingCtx, stop := context.WithCancel(ctx)
			cancel = stop
			ready := make(chan struct{}, 1)
			done := make(chan recordingResult, 1)
			started, finished = ready, done
			go func() {
				audio, err := record(recordingCtx, func() { ready <- struct{}{} })
				done <- recordingResult{audio: audio, err: err}
			}()
		case <-started:
			started = nil
			state = microphoneListening
			if err := c.updateMicrophone(ctx, state, "正在听，请说话；再次点击麦克风结束"); err != nil {
				return err
			}
		case result := <-finished:
			cancel()
			cancel, started, finished = nil, nil, nil
			state = microphoneIdle
			if ctx.Err() != nil {
				return nil
			}
			message := recordingMessage(result, save)
			if err := c.updateMicrophone(ctx, state, message); err != nil {
				return err
			}
		}
	}
}

// recordingMessage 在后台保存本次录音，并把录音或保存失败转换成界面提示。
func recordingMessage(result recordingResult, save func([]byte) (string, error)) string {
	if result.err == nil {
		filename, err := save(result.audio)
		if err == nil {
			log.Printf("录音已保存：%s", filename)
			return "录音已结束，语音识别服务尚未配置"
		}
		result.err = fmt.Errorf("保存录音: %w", err)
	}
	if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
		return "本次未录到声音，请再次点击麦克风"
	}
	log.Printf("录音失败：%v", result.err)
	return "录音失败：" + result.err.Error()
}

// updateMicrophone 通过主循环同时更新按钮和底部状态，不另加文字区域。
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
		return c.showText(message, false)
	})
}
