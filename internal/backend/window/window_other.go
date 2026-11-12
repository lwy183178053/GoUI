//go:build !windows && !linux

package window

import (
	"errors"

	"github.com/lwy183178053/GoUI/internal/backend"
)

func Displays() ([]Display, error) {
	return nil, errors.New("desktop display enumeration is supported on Windows and Linux only")
}

func (p *presenter) Run(backend.FrameSource, func() error) error {
	return errors.New("desktop windows are supported on Windows and Linux only")
}

func (p *presenter) Minimize() error {
	return errors.New("desktop windows are supported on Windows and Linux only")
}

func (p *presenter) ToggleMaximize() error {
	return errors.New("desktop windows are supported on Windows and Linux only")
}

func (p *presenter) Close() error {
	return errors.New("desktop windows are supported on Windows and Linux only")
}
