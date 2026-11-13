package goui

import (
	"errors"
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/lwy183178053/GoUI/internal/ui"
)

type testPageControls struct {
	imageSource string
	finish      func() error
	kinds       map[string]ui.Kind
	text        string
	fill        color.NRGBA
	textColor   color.NRGBA
	visible     bool
	disabled    bool
	events      map[ui.ButtonEvent]func() error
	move        [2]int
	moveTime    time.Duration
	rotation    float64
	rotateTime  time.Duration
	scale       [2]float64
	scaleTime   time.Duration
}

func (p *testPageControls) Kind(id string) (ui.Kind, error) {
	kind, ok := p.kinds[id]
	if !ok {
		return "", fmt.Errorf("unknown UI id %q", id)
	}
	return kind, nil
}
func (p *testPageControls) SetText(_ string, value string) error { p.text = value; return nil }
func (p *testPageControls) SetColor(_ string, value color.NRGBA) error {
	p.fill = value
	return nil
}
func (p *testPageControls) SetTextColor(_ string, value color.NRGBA) error {
	p.textColor = value
	return nil
}
func (p *testPageControls) SetVisible(_ string, value bool) error { p.visible = value; return nil }
func (p *testPageControls) SetDisabled(_ string, value bool) error {
	p.disabled = value
	return nil
}
func (p *testPageControls) SetImageSource(_ string, src string) error {
	p.imageSource = src
	return nil
}
func (p *testPageControls) FinishImage(_ string, callback func() error) error {
	p.finish = callback
	return nil
}
func (p *testPageControls) SetButtonEvent(_ string, event ui.ButtonEvent, callback func() error) error {
	if p.events == nil {
		p.events = make(map[ui.ButtonEvent]func() error)
	}
	if p.events[event] != nil {
		return errors.New("already bound")
	}
	p.events[event] = callback
	return nil
}
func (p *testPageControls) MoveTo(_ string, x, y int, duration time.Duration) error {
	p.move, p.moveTime = [2]int{x, y}, duration
	return nil
}
func (p *testPageControls) RotateTo(_ string, degrees float64, duration time.Duration) error {
	p.rotation, p.rotateTime = degrees, duration
	return nil
}
func (p *testPageControls) ScaleTo(_ string, x, y float64, duration time.Duration) error {
	p.scale, p.scaleTime = [2]float64{x, y}, duration
	return nil
}

func TestTypedComponentGettersAndMethods(t *testing.T) {
	controls := &testPageControls{kinds: map[string]ui.Kind{
		"panel": ui.Block, "title": ui.Text, "image": ui.Image, "run": ui.Button,
	}}
	page := &Page{renderer: controls}
	block, err := page.GetBlock("panel")
	if err != nil {
		t.Fatal(err)
	}
	text, err := page.GetText("title")
	if err != nil {
		t.Fatal(err)
	}
	imageNode, err := page.GetImage("image")
	if err != nil {
		t.Fatal(err)
	}
	button, err := page.GetButton("run")
	if err != nil {
		t.Fatal(err)
	}
	fill := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	textColor := color.NRGBA{R: 220, G: 230, B: 240, A: 255}
	if err := block.SetColor(fill); err != nil {
		t.Fatal(err)
	}
	if err := block.SetVisible(false); err != nil {
		t.Fatal(err)
	}
	if err := text.SetText("hello"); err != nil {
		t.Fatal(err)
	}
	if err := text.SetTextColor(textColor); err != nil {
		t.Fatal(err)
	}
	if err := imageNode.SetVisible(true); err != nil {
		t.Fatal(err)
	}
	if err := button.SetDisabled(true); err != nil {
		t.Fatal(err)
	}
	if controls.fill != fill || controls.visible != true || controls.text != "hello" || controls.textColor != textColor || !controls.disabled {
		t.Fatalf("component methods did not forward their values: %#v", controls)
	}
}

func TestGetterReportsMissingAndWrongType(t *testing.T) {
	page := &Page{renderer: &testPageControls{kinds: map[string]ui.Kind{"title": ui.Text}}}
	if _, err := page.GetButton("missing"); err == nil {
		t.Fatal("missing id unexpectedly resolved")
	}
	if _, err := page.GetButton("title"); err == nil || !strings.Contains(err.Error(), "not button") {
		t.Fatalf("wrong component type returned %v", err)
	}
}

func TestButtonOnClickReceivesPage(t *testing.T) {
	controls := &testPageControls{kinds: map[string]ui.Kind{"run": ui.Button}}
	page := &Page{renderer: controls}
	button, err := page.GetButton("run")
	if err != nil {
		t.Fatal(err)
	}
	called := false
	if err := button.OnClick(func(got *Page) error {
		called = got == page
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if controls.events[ui.Click] == nil {
		t.Fatal("button callback was not registered")
	}
	if err := controls.events[ui.Click](); err != nil || !called {
		t.Fatalf("callback received the wrong page or returned an error: called=%v err=%v", called, err)
	}
	if err := button.OnClick(func(*Page) error { return nil }); err == nil {
		t.Fatal("duplicate callback unexpectedly succeeded")
	}
}

func TestButtonMouseEventsAndBlockAnimationsForward(t *testing.T) {
	controls := &testPageControls{kinds: map[string]ui.Kind{"card": ui.Block, "run": ui.Button}}
	page := &Page{renderer: controls}
	button, err := page.GetButton("run")
	if err != nil {
		t.Fatal(err)
	}
	called := ui.ButtonEvent(255)
	if err := button.OnMouseEnter(func(*Page) error { called = ui.MouseEnter; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := button.OnMouseDown(func(*Page) error { called = ui.MouseDown; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := button.OnMouseUp(func(*Page) error { called = ui.MouseUp; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := button.OnMouseLeave(func(*Page) error { called = ui.MouseLeave; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, event := range []ui.ButtonEvent{ui.MouseEnter, ui.MouseDown, ui.MouseUp, ui.MouseLeave} {
		if err := controls.events[event](); err != nil || called != event {
			t.Fatalf("button event %d was not forwarded: called=%d err=%v", event, called, err)
		}
	}

	block, err := page.GetBlock("card")
	if err != nil {
		t.Fatal(err)
	}
	duration := 250 * time.Millisecond
	if err := block.MoveTo(12, -8, duration); err != nil {
		t.Fatal(err)
	}
	if err := block.RotateTo(90, duration); err != nil {
		t.Fatal(err)
	}
	if err := block.ScaleTo(2, 0.5, duration); err != nil {
		t.Fatal(err)
	}
	if controls.move != [2]int{12, -8} || controls.moveTime != duration || controls.rotation != 90 || controls.rotateTime != duration || controls.scale != [2]float64{2, 0.5} || controls.scaleTime != duration {
		t.Fatalf("animation arguments were not forwarded: %#v", controls)
	}
}

func TestPageWithoutOpenApplicationReturnsError(t *testing.T) {
	if _, err := (*Page)(nil).GetText("title"); err == nil {
		t.Fatal("nil page unexpectedly returned a component")
	}
	if err := (*Block)(nil).SetColor(color.NRGBA{}); err == nil {
		t.Fatal("nil component unexpectedly accepted an update")
	}
	if err := (*Application)(nil).Run(); err == nil {
		t.Fatal("nil application unexpectedly ran")
	}
}
