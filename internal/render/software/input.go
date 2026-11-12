package software

import (
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/lwy183178053/GoUI/internal/layout"
	"github.com/lwy183178053/GoUI/internal/ui"
)

// SetButtonEvent 为指定按钮注册一个鼠标事件。
func (r *Renderer) SetButtonEvent(id string, event ui.ButtonEvent, callback func() error) error {
	if r.closed {
		return errors.New("renderer is closed")
	}
	if callback == nil {
		return errors.New("button event callback is required")
	}
	node, _, err := r.node(id)
	if err != nil {
		return err
	}
	if node.Button == nil {
		return fmt.Errorf("UI id %q is not a button", id)
	}
	if event > ui.Click {
		return fmt.Errorf("unknown button event %d", event)
	}
	if r.handlers[id] == nil {
		r.handlers[id] = make(map[ui.ButtonEvent]func() error)
	}
	if _, exists := r.handlers[id][event]; exists {
		return fmt.Errorf("button %q already has this event callback", id)
	}
	r.handlers[id][event] = callback
	return nil
}

func (r *Renderer) dispatch(node *ui.Node, event ui.ButtonEvent) error {
	if node == nil || node.Button == nil || node.Button.Disabled {
		return nil
	}
	if callback := r.handlers[node.ID][event]; callback != nil {
		return callback()
	}
	return nil
}

// PointerMove 在鼠标目标变化时派发进入和移出事件。
func (r *Renderer) PointerMove(x, y int) error {
	target := r.hitButton(x, y)
	if target != nil && target.Button.Disabled {
		target = nil
	}
	if target == r.hovered {
		return nil
	}
	previous := r.hovered
	r.hovered = target
	if err := r.dispatch(previous, ui.MouseLeave); err != nil {
		return err
	}
	return r.dispatch(target, ui.MouseEnter)
}

// PointerDown 记录按下目标并派发鼠标按下事件。
func (r *Renderer) PointerDown(x, y int) error {
	target := r.hitButton(x, y)
	if target != nil && target.Button.Disabled {
		r.pressed = nil
		return nil
	}
	r.pressed = target
	return r.dispatch(r.pressed, ui.MouseDown)
}

// PointerUp 派发鼠标抬起；只有在原按钮上释放才会继续触发点击。
func (r *Renderer) PointerUp(x, y int) error {
	pressed := r.pressed
	r.pressed = nil
	target := r.hitButton(x, y)
	if err := r.dispatch(target, ui.MouseUp); err != nil {
		return err
	}
	if pressed == nil || target != pressed || pressed.Button.Disabled {
		return nil
	}
	return r.dispatch(pressed, ui.Click)
}

func (r *Renderer) hitButton(x, y int) *ui.Node {
	return r.hitBox(r.layout, float64(x), float64(y))
}

func (r *Renderer) hitBox(box *layout.Box, x, y float64) *ui.Node {
	if box == nil || !box.Node.Visible {
		return nil
	}
	for i := len(box.Children) - 1; i >= 0; i-- {
		if node := r.hitBox(box.Children[i], x, y); node != nil {
			return node
		}
	}
	if box.Node.Button == nil {
		return nil
	}
	localX, localY := inversePoint(r.world[box.Node], x, y)
	if image.Pt(int(math.Floor(localX)), int(math.Floor(localY))).In(box.Pixels) {
		return box.Node
	}
	return nil
}
