package main

import (
	"fmt"
	"image/color"
	"log"
	"time"

	goui "github.com/lwy183178053/GoUI"
)

func main() {
	app, err := goui.Open("./ui")
	check(err)

	page := app.Page()
	status, err := page.GetText("demo-status")
	check(err)
	hint, err := page.GetText("hidden-hint")
	check(err)
	runButton, err := page.GetButton("run-button")
	check(err)
	toggleButton, err := page.GetButton("toggle-button")
	check(err)
	moveButton, err := page.GetButton("move-button")
	check(err)
	rotateButton, err := page.GetButton("rotate-button")
	check(err)
	scaleButton, err := page.GetButton("scale-button")
	check(err)
	minimizeButton, err := page.GetButton("window-minimize")
	check(err)
	maximizeButton, err := page.GetButton("window-maximize")
	check(err)
	closeButton, err := page.GetButton("window-close")
	check(err)
	target, err := page.GetBlock("motion-target")
	check(err)

	baseColor := color.NRGBA{R: 41, G: 126, B: 116, A: 255}
	disabledColor := color.NRGBA{R: 83, G: 92, B: 92, A: 255}
	runCount := 0
	runDisabled := false
	moved, rotated, enlarged := false, false, false
	duration := 500 * time.Millisecond

	check(runButton.OnMouseEnter(func(*goui.Page) error {
		return status.SetText("鼠标已进入运行按钮")
	}))
	check(runButton.OnMouseLeave(func(*goui.Page) error {
		return status.SetText("鼠标已离开运行按钮")
	}))
	check(runButton.OnMouseDown(func(*goui.Page) error {
		return status.SetText("鼠标已按下")
	}))
	check(runButton.OnMouseUp(func(*goui.Page) error {
		return status.SetText("鼠标已抬起")
	}))
	check(runButton.OnClick(func(*goui.Page) error {
		runCount++
		return status.SetText(fmt.Sprintf("运行按钮已点击 %d 次", runCount))
	}))

	check(toggleButton.OnClick(func(*goui.Page) error {
		runDisabled = !runDisabled
		if err := runButton.SetDisabled(runDisabled); err != nil {
			return err
		}
		if runDisabled {
			if err := runButton.SetColor(disabledColor); err != nil {
				return err
			}
			if err := hint.SetVisible(true); err != nil {
				return err
			}
			return status.SetText("运行按钮已禁用")
		}
		if err := runButton.SetColor(baseColor); err != nil {
			return err
		}
		if err := hint.SetVisible(false); err != nil {
			return err
		}
		return status.SetText("运行按钮已启用")
	}))

	check(moveButton.OnClick(func(*goui.Page) error {
		moved = !moved
		y := 20
		if moved {
			y = 54
		}
		if err := target.MoveTo(826, y, duration); err != nil {
			return err
		}
		return status.SetText("移动动画已启动")
	}))
	check(rotateButton.OnClick(func(*goui.Page) error {
		rotated = !rotated
		degrees := 0.0
		if rotated {
			degrees = 90
		}
		if err := target.RotateTo(degrees, duration); err != nil {
			return err
		}
		return status.SetText("旋转动画已启动")
	}))
	check(scaleButton.OnClick(func(*goui.Page) error {
		enlarged = !enlarged
		scale := 1.0
		if enlarged {
			scale = 1.5
		}
		if err := target.ScaleTo(scale, scale, duration); err != nil {
			return err
		}
		return status.SetText("缩放动画已启动")
	}))
	check(minimizeButton.OnClick(func(*goui.Page) error {
		return app.MinimizeWindow()
	}))
	check(maximizeButton.OnClick(func(*goui.Page) error {
		return app.ToggleMaximizeWindow()
	}))
	check(closeButton.OnClick(func(*goui.Page) error {
		return app.CloseWindow()
	}))

	check(app.Run())
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
