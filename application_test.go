package goui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func makeUITestDirectory(t *testing.T, config string) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "fonts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"config.xml":        []byte(config),
		"main.xml":          []byte(`<ui><text id="status" size="100,24">Ready</text><button id="run" size="80,30"/></ui>`),
		"fonts/default.ttf": goregular.TTF,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(directory, filepath.FromSlash(name)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func TestOpenLoadsUIAndDefaults(t *testing.T) {
	directory := makeUITestDirectory(t, `<config><fonts><font name="default" src="fonts/default.ttf"/></fonts></config>`)
	app, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	if app.Page() == nil || app.document.Window.Backend != "desktop" || app.document.Window.Display != 0 || app.document.Window.Width != 960 || app.document.Window.Height != 640 || app.document.Window.Fullscreen || !app.document.Window.Decorated || app.document.Window.Resizable || app.document.Window.MinWidth != 0 || app.document.Window.MinHeight != 0 {
		t.Fatalf("unexpected application defaults: %#v", app.document.Window)
	}
	status, err := app.Page().GetText("status")
	if err != nil {
		t.Fatal(err)
	}
	if err := status.SetText("loaded"); err != nil {
		t.Fatal(err)
	}
	if err := app.renderer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenParsesWindowSettings(t *testing.T) {
	config := `<config><window backend="desktop" display="1" width="1280" height="720" fullscreen="true" decorated="false" resizable="true" min-width="640" min-height="480"/><fonts><font name="default" src="fonts/default.ttf"/></fonts></config>`
	app, err := Open(makeUITestDirectory(t, config))
	if err != nil {
		t.Fatal(err)
	}
	got := app.document.Window
	if got.Backend != "desktop" || got.Display != 1 || got.Width != 1280 || got.Height != 720 || !got.Fullscreen || got.Decorated || !got.Resizable || got.MinWidth != 640 || got.MinHeight != 480 {
		t.Fatalf("window settings = %#v", got)
	}
	_ = app.renderer.Close()
}

func TestWindowOperationsRequireRunningWindow(t *testing.T) {
	app, err := Open(makeUITestDirectory(t, `<config><fonts><font name="default" src="fonts/default.ttf"/></fonts></config>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func() error{app.MinimizeWindow, app.ToggleMaximizeWindow, app.CloseWindow} {
		if err := operation(); err == nil || !strings.Contains(err.Error(), "window is not running") {
			t.Errorf("window operation returned %v before Run", err)
		}
	}
	_ = app.renderer.Close()
}

func TestDRMBackendReturnsNotImplementedError(t *testing.T) {
	config := `<config><window backend="drm"/><fonts><font name="default" src="fonts/default.ttf"/></fonts></config>`
	app, err := Open(makeUITestDirectory(t, config))
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(); err == nil || !strings.Contains(err.Error(), "DRM backend is not implemented") {
		t.Fatalf("Run returned %v", err)
	}
	if err := app.Run(); err == nil || !strings.Contains(err.Error(), "only be run once") {
		t.Fatalf("second Run returned %v", err)
	}
}

func TestOpenReportsMissingUIDirectory(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Open unexpectedly loaded a missing directory")
	}
}
