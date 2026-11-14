package voice

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Linux 复用 ALSA 的 aplay；默认设备由系统配置，Go 交叉编译仍无需 CGO。
func playAudio(ctx context.Context, wav []byte, _ pcmAudio) error {
	path, err := exec.LookPath("aplay")
	if err != nil {
		return fmt.Errorf("Linux 播放需要 aplay，请安装 alsa-utils: %w", err)
	}
	command := exec.CommandContext(ctx, path, "-q", "-t", "wav")
	command.Stdin = bytes.NewReader(wav)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("aplay: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return ctx.Err()
}
