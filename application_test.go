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
	if app.Page() == nil || app.document.Window.Backend != "desktop" || app.document.Window.Display != 0 || app.document.Window.Width != 960 || app.document.Window.Height != 640 || app.document.Window.Fullscreen {
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
	config := `<config><window backend="desktop" display="1" width="1280" height="720" fullscreen="true"/><fonts><font name="default" src="fonts/default.ttf"/></fonts></config>`
	app, err := Open(makeUITestDirectory(t, config))
	if err != nil {
		t.Fatal(err)
	}
	got := app.document.Window
	if got.Backend != "desktop" || got.Display != 1 || got.Width != 1280 || got.Height != 720 || !got.Fullscreen {
		t.Fatalf("window settings = %#v", got)
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
