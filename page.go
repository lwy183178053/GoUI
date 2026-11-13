package goui

import (
	"errors"
	"fmt"
	"image/color"
	"time"

	"github.com/lwy183178053/GoUI/internal/ui"
)

type pageControls interface {
	Kind(string) (ui.Kind, error)
	SetText(string, string) error
	SetColor(string, color.NRGBA) error
	SetTextColor(string, color.NRGBA) error
	SetVisible(string, bool) error
	SetDisabled(string, bool) error
	SetButtonEvent(string, ui.ButtonEvent, func() error) error
	MoveTo(string, int, int, time.Duration) error
	RotateTo(string, float64, time.Duration) error
	ScaleTo(string, float64, float64, time.Duration) error
}

// Page 用 XML 中的 id 查找组件。
// 组件方法在窗口主循环中调用；外部 goroutine 使用 Application.Dispatch。
type Page struct {
	renderer pageControls
}

func (p *Page) target() (pageControls, error) {
	if p == nil || p.renderer == nil {
		return nil, errors.New("page is not connected to an open application")
	}
	return p.renderer, nil
}

func (p *Page) get(id string, kind ui.Kind) (*Block, error) {
	target, err := p.target()
	if err != nil {
		return nil, err
	}
	actual, err := target.Kind(id)
	if err != nil {
		return nil, err
	}
	if actual != kind {
		return nil, fmt.Errorf("UI id %q is %s, not %s", id, actual, kind)
	}
	return &Block{page: p, id: id}, nil
}

// GetBlock 返回指定的 Block 组件。
func (p *Page) GetBlock(id string) (*Block, error) { return p.get(id, ui.Block) }

// GetText 返回指定的 Text 组件。
func (p *Page) GetText(id string) (*Text, error) {
	block, err := p.get(id, ui.Text)
	if err != nil {
		return nil, err
	}
	return &Text{Block: *block}, nil
}

// GetImage 返回指定的 Image 组件。
func (p *Page) GetImage(id string) (*Image, error) {
	block, err := p.get(id, ui.Image)
	if err != nil {
		return nil, err
	}
	return &Image{Block: *block}, nil
}

// GetButton 返回指定的 Button 组件。
func (p *Page) GetButton(id string) (*Button, error) {
	block, err := p.get(id, ui.Button)
	if err != nil {
		return nil, err
	}
	return &Button{Block: *block}, nil
}

// Block 是所有可视组件共有的基础类型。
type Block struct {
	page *Page
	id   string
}

func (b *Block) target() (pageControls, error) {
	if b == nil {
		return nil, errors.New("component is not connected to an open application")
	}
	return b.page.target()
}

// SetColor 修改当前组件的填充色。
func (b *Block) SetColor(value color.NRGBA) error {
	target, err := b.target()
	if err != nil {
		return err
	}
	return target.SetColor(b.id, value)
}

// SetVisible 显示或隐藏当前组件及其子节点。
func (b *Block) SetVisible(visible bool) error {
	target, err := b.target()
	if err != nil {
		return err
	}
	return target.SetVisible(b.id, visible)
}

// MoveTo 在指定时长内移动到父组件坐标中的目标位置；时长为 0 时立即移动。
func (b *Block) MoveTo(x, y int, duration time.Duration) error {
	target, err := b.target()
	if err != nil {
		return err
	}
	return target.MoveTo(b.id, x, y, duration)
}

// RotateTo 在指定时长内绕当前组件的 pivot 顺时针旋转到目标角度。
func (b *Block) RotateTo(degrees float64, duration time.Duration) error {
	target, err := b.target()
	if err != nil {
		return err
	}
	return target.RotateTo(b.id, degrees, duration)
}

// ScaleTo 在指定时长内围绕当前组件的 pivot 缩放，1 表示原始大小。
func (b *Block) ScaleTo(x, y float64, duration time.Duration) error {
	target, err := b.target()
	if err != nil {
		return err
	}
	return target.ScaleTo(b.id, x, y, duration)
}

// Text 是可修改内容和文字颜色的文本组件。
type Text struct{ Block }

// SetText 修改文本内容。
func (t *Text) SetText(value string) error {
	target, err := t.target()
	if err != nil {
		return err
	}
	return target.SetText(t.id, value)
}

// SetTextColor 修改文字颜色。
func (t *Text) SetTextColor(value color.NRGBA) error {
	target, err := t.target()
	if err != nil {
		return err
	}
	return target.SetTextColor(t.id, value)
}

// Image 是可显示和隐藏的图片组件。
type Image struct{ Block }

// Button 是可绑定鼠标事件的按钮组件。
type Button struct{ Block }

// ButtonEventHandler 是按钮鼠标事件的回调。
type ButtonEventHandler func(*Page) error

// SetDisabled 设置按钮是否响应点击。
func (b *Button) SetDisabled(disabled bool) error {
	target, err := b.target()
	if err != nil {
		return err
	}
	return target.SetDisabled(b.id, disabled)
}

func (b *Button) on(event ui.ButtonEvent, callback ButtonEventHandler) error {
	if callback == nil {
		return errors.New("button event callback is required")
	}
	target, err := b.target()
	if err != nil {
		return err
	}
	return target.SetButtonEvent(b.id, event, func() error { return callback(b.page) })
}

// OnMouseEnter 在鼠标进入按钮时调用。
func (b *Button) OnMouseEnter(callback ButtonEventHandler) error {
	return b.on(ui.MouseEnter, callback)
}

// OnMouseLeave 在鼠标离开按钮时调用。
func (b *Button) OnMouseLeave(callback ButtonEventHandler) error {
	return b.on(ui.MouseLeave, callback)
}

// OnMouseDown 在按钮上按下鼠标左键时调用。
func (b *Button) OnMouseDown(callback ButtonEventHandler) error {
	return b.on(ui.MouseDown, callback)
}

// OnMouseUp 在鼠标左键释放且指针位于按钮上时调用。
func (b *Button) OnMouseUp(callback ButtonEventHandler) error {
	return b.on(ui.MouseUp, callback)
}

// OnClick 在按钮上按下并在同一按钮上释放鼠标左键时调用。
func (b *Button) OnClick(callback ButtonEventHandler) error {
	return b.on(ui.Click, callback)
}
