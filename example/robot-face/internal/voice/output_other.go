//go:build !windows && !linux

package voice

import (
	"context"
	"errors"
)

func playAudio(context.Context, []byte, pcmAudio) error {
	return errors.New("语音播放目前支持 Windows 和 Linux")
}
