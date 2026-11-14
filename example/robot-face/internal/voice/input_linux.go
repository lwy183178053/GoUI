package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Linux 先复用 ALSA 的 arecord，不给 Go 程序引入 CGO。
// 默认录音设备由系统配置；缺少工具、权限或设备时，将原始错误传给界面。
type linuxInput struct {
	command *exec.Cmd
	output  io.ReadCloser
	stderr  bytes.Buffer
	cancel  context.CancelFunc
	closed  bool
	waitErr error
}

func openInput(ctx context.Context) (io.ReadCloser, error) {
	commandPath, err := exec.LookPath("arecord")
	if err != nil {
		return nil, fmt.Errorf("Linux 录音需要 arecord，请安装 alsa-utils: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	input := &linuxInput{cancel: cancel}
	input.command = exec.CommandContext(ctx, commandPath,
		"-q", "-t", "raw", "-f", "S16_LE", "-r", "16000", "-c", "1",
		"--period-size=320", "--buffer-size=1280")
	input.command.Stderr = &input.stderr
	input.command.WaitDelay = time.Second
	input.output, err = input.command.StdoutPipe()
	if err == nil {
		err = input.command.Start()
	}
	if err != nil {
		cancel()
		if input.output != nil {
			input.output.Close()
		}
		return nil, fmt.Errorf("打开麦克风: %w", err)
	}
	return input, nil
}

func (input *linuxInput) Read(buffer []byte) (int, error) {
	n, err := input.output.Read(buffer)
	if err != nil {
		// 等进程退出后再读取 stderr，避免与命令的输出 goroutine 并发访问。
		input.Close()
		if detail := strings.TrimSpace(input.stderr.String()); detail != "" {
			err = fmt.Errorf("arecord: %s", detail)
		} else if input.waitErr != nil {
			err = fmt.Errorf("arecord: %w", input.waitErr)
		}
	}
	return n, err
}

func (input *linuxInput) Close() error {
	if input.closed {
		return nil
	}
	input.closed = true
	input.cancel()
	input.output.Close()
	input.waitErr = input.command.Wait()
	return nil
}
