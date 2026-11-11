package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"os"

	"goui/internal/backend/window"
	"goui/internal/render/software"
	"goui/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "goui: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("goui", flag.ContinueOnError)
	directory := flags.String("ui", "./GoUI", "UI directory containing config.xml and main.xml")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q; use -ui to select a UI directory", flags.Arg(0))
	}
	document, err := ui.Load(*directory)
	if err != nil {
		return err
	}
	renderer, err := software.New(document)
	if err != nil {
		return err
	}
	defer renderer.Close()

	clicks := 0
	if err := renderer.RegisterAction("preview.run", func() error {
		clicks++
		return renderer.SetText("demo-status", fmt.Sprintf("Go 命令已执行 %d 次", clicks))
	}); err != nil {
		return err
	}
	runDisabled, showHint := true, false
	if err := renderer.RegisterAction("preview.toggle", func() error {
		runDisabled = !runDisabled
		showHint = !showHint
		if err := renderer.SetDisabled("run-button", runDisabled); err != nil {
			return err
		}
		buttonColor := color.NRGBA{R: 41, G: 126, B: 116, A: 255}
		if runDisabled {
			buttonColor = color.NRGBA{R: 83, G: 92, B: 92, A: 255}
		}
		if err := renderer.SetColor("run-button", buttonColor); err != nil {
			return err
		}
		if err := renderer.SetVisible("hidden-hint", showHint); err != nil {
			return err
		}
		if runDisabled {
			return renderer.SetText("demo-status", "运行按钮已禁用；再次点击可启用")
		}
		return renderer.SetText("demo-status", "运行按钮已启用；可以触发 Go 命令")
	}); err != nil {
		return err
	}
	return window.New(window.Options{Title: "GoUI Preview"}).Run(renderer)
}
