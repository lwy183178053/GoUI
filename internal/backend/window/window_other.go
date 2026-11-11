//go:build !windows

package window

import (
	"errors"

	"goui/internal/backend"
)

func (p *presenter) Run(backend.FrameSource) error {
	return errors.New("the preview window is supported on Windows only; XML, layout and software rendering are platform-independent")
}
