//go:build !windows && !linux

package voice

import (
	"context"
	"errors"
	"io"
)

func openInput(context.Context) (io.ReadCloser, error) {
	return nil, errors.New("麦克风录音目前支持 Windows 和 Linux")
}
