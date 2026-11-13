package ui

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/image/font/gofont/goregular"
)

const testConfig = `<config><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`

func fixture(t *testing.T, config, main string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "fonts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"config.xml": []byte(config), "main.xml": []byte(main), "fonts/regular.ttf": goregular.TTF,
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadDocument(t *testing.T) {
	main := `<?xml version="1.0" encoding="UTF-8"?>
<ui id="root" color="#10203080">
  <!-- A block contributes coordinates, but does not inherit text styles. -->
  <block id="panel" anchor="0.5,1" pivot="0.5,1" position="-4,8" size="-1,80" margin="2,3,4,5" radius="7.5" color="#AABBCC">
    <text id="title" size="100,30" font-size="20.5" text-color="#FFEEDD80" align="center" valign="top" wrap="true"> Hello &amp; &lt;Go&gt; </text>
  </block>
  <text id="defaults" size="0,0"/>
</ui>`
	dir := fixture(t, testConfig, main)
	doc, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Directory != dir || len(doc.Fonts) != 1 || doc.Fonts["default"] == nil {
		t.Fatalf("wrong directory or fonts: %#v", doc)
	}
	if doc.Window != (WindowConfig{Backend: "desktop", Width: 960, Height: 640, Decorated: true}) {
		t.Fatalf("wrong default window settings: %#v", doc.Window)
	}
	if doc.Root.Color != (color.NRGBA{R: 16, G: 32, B: 48, A: 128}) || doc.Find("root") != doc.Root {
		t.Fatal("root color or lookup is incorrect")
	}
	panel := doc.Find("panel")
	if panel.Kind != Block || panel.Anchor != (Vec2{0.5, 1}) || panel.Pivot != (Vec2{0.5, 1}) || panel.Position != image.Pt(-4, 8) || panel.Size != image.Pt(-1, 80) || panel.Margin != (Insets{2, 3, 4, 5}) || panel.Radius != 7.5 {
		t.Fatalf("block attributes were not preserved: %#v", panel)
	}
	text := doc.Find("title").Text
	if text.Content != "Hello & <Go>" || text.Font != "default" || text.FontSize != 20.5 || !text.Wrap || text.Align != "center" || text.VAlign != "top" || text.Color.A != 128 {
		t.Fatalf("wrong text: %#v", text)
	}
	defaults := doc.Find("defaults")
	if !defaults.Visible || defaults.Color.A != 0 || defaults.Radius != 0 || defaults.Text.Wrap || defaults.Text.Font != "default" || defaults.Text.FontSize != 16 || defaults.Text.Align != "left" || defaults.Text.VAlign != "middle" {
		t.Fatalf("wrong defaults: %#v", defaults)
	}
	if doc.Find("missing") != nil || defaults.Source.Line != 7 || defaults.Source.Column != 3 {
		t.Fatalf("lookup or source location is incorrect: %#v", defaults.Source)
	}
}

func TestLoadingErrorsHaveLocations(t *testing.T) {
	tests := []struct{ name, config, main, want string }{
		{"missing default", `<config><fonts/></config>`, `<ui/>`, `named "default"`},
		{"duplicate font", `<config><fonts><font name="default" src="fonts/regular.ttf"/><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `duplicate font name`},
		{"empty font name", `<config><fonts><font name=" " src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `non-empty "name"`},
		{"missing font src", `<config><fonts><font name="default"/></fonts></config>`, `<ui/>`, `non-empty "src"`},
		{"missing font file", `<config><fonts><font name="default" src="missing.ttf"/></fonts></config>`, `<ui/>`, `read font`},
		{"bad font file", `<config><fonts><font name="default" src="main.xml"/></fonts></config>`, `<ui/>`, `parse font`},
		{"unknown config tag", `<config><other/></config>`, `<ui/>`, `<fonts>`},
		{"unknown window backend", `<config><window backend="gpu"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `backend must be desktop or drm`},
		{"negative display index", `<config><window display="-1"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `display must be an integer`},
		{"zero window width", `<config><window width="0"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `width must be an integer`},
		{"invalid fullscreen", `<config><window fullscreen="yes"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `fullscreen must be true or false`},
		{"invalid decorated", `<config><window decorated="yes"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `decorated must be true or false`},
		{"invalid resizable", `<config><window resizable="yes"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `resizable must be true or false`},
		{"zero minimum width", `<config><window min-width="0"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `min-width must be a positive integer`},
		{"minimum width exceeds window", `<config><window width="800" min-width="900"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `min-width cannot exceed width`},
		{"minimum height exceeds window", `<config><window height="600" min-height="700"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `min-height cannot exceed height`},
		{"duplicate window", `<config><window/><window/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `only one <window>`},
		{"window children", `<config><window><child/></window><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`, `<ui/>`, `cannot contain child`},
		{"unknown config attribute", `<config font="default"><fonts/></config>`, `<ui/>`, `unsupported attribute`},
		{"font children", `<config><fonts><font name="default" src="fonts/regular.ttf"><font/></font></fonts></config>`, `<ui/>`, `cannot contain child`},
		{"unknown font", testConfig, `<ui><text size="10,10" font="system">Hello</text></ui>`, `unregistered font`},
		{"hidden unknown font", testConfig, `<ui><text size="10,10" visible="false" font="missing"/></ui>`, `unregistered font`},
		{"root font", testConfig, `<ui font="default"/>`, `unsupported attribute "font"`},
		{"root size", testConfig, `<ui size="10,10"/>`, `unsupported attribute "size"`},
		{"duplicate id", testConfig, "<ui id=\"x\">\n  <block id=\"x\" size=\"1,1\"/>\n</ui>", `duplicate id`},
		{"empty id", testConfig, `<ui id=" "/>`, `id must not be empty`},
		{"unknown tag", testConfig, `<ui><widget size="1,1"/></ui>`, `unsupported tag <widget>`},
		{"removed face tag", testConfig, `<ui><face size="1,1" src="robot.xml"/></ui>`, `unsupported tag <face>`},
		{"image invalid fit", testConfig, `<ui><image size="1,1" src="a.png" fit="stretchy"/></ui>`, `fit must be`},
		{"button invalid disabled", testConfig, `<ui><button size="1,1" disabled="yes"/></ui>`, `disabled must be`},
		{"button invalid hover color", testConfig, `<ui><button size="1,1" hover-color="red"/></ui>`, `hover-color must be`},
		{"hover color is button-only", testConfig, `<ui><block size="1,1" hover-color="#FFFFFF"/></ui>`, `unsupported attribute "hover-color"`},
		{"include not supported", testConfig, `<ui><include src="x.xml"/></ui>`, `unsupported tag <include>`},
		{"nested ui", testConfig, `<ui><ui/></ui>`, `unsupported tag <ui>`},
		{"unknown attribute", testConfig, `<ui><block size="1,1" background="#FFFFFF"/></ui>`, `unsupported attribute "background"`},
		{"old button action", testConfig, `<ui><button size="1,1" on-click="click"/></ui>`, `unsupported attribute "on-click"`},
		{"text child", testConfig, `<ui><text size="10,10"><block size="1,1"/></text></ui>`, `<text> may contain text only`},
		{"block text", testConfig, `<ui><block size="1,1">invalid</block></ui>`, `cannot contain text`},
		{"invalid root", testConfig, `<block size="1,1"/>`, `<ui> root`},
		{"missing size", testConfig, `<ui><block/></ui>`, `non-empty "size"`},
		{"short size", testConfig, `<ui><block size="-1"/></ui>`, `two integers`},
		{"negative size", testConfig, `<ui><block size="-2,1"/></ui>`, `two integers`},
		{"fractional size", testConfig, `<ui><block size="1.5,1"/></ui>`, `two integers`},
		{"fractional position", testConfig, `<ui><block size="1,1" position="1.5,0"/></ui>`, `integer pixels`},
		{"bad anchor", testConfig, `<ui><block size="1,1" anchor="1.1,0"/></ui>`, `between 0 and 1`},
		{"nan pivot", testConfig, `<ui><block size="1,1" pivot="NaN,0"/></ui>`, `finite numbers`},
		{"bad margin", testConfig, `<ui><block size="1,1" margin="0,0,-1,0"/></ui>`, `non-negative`},
		{"short margin", testConfig, `<ui><block size="1,1" margin="1,2"/></ui>`, `four non-negative`},
		{"fractional margin", testConfig, `<ui><block size="1,1" margin="1,2,3,4.5"/></ui>`, `integer pixels`},
		{"invalid color", testConfig, `<ui color="#XYZXYZ"/>`, `#RRGGBB`},
		{"short color", testConfig, `<ui color="#FFF"/>`, `#RRGGBB`},
		{"invalid text color", testConfig, `<ui><text size="1,1" text-color="red"/></ui>`, `text-color`},
		{"negative radius", testConfig, `<ui><block size="1,1" radius="-1"/></ui>`, `radius`},
		{"infinite radius", testConfig, `<ui><block size="1,1" radius="Inf"/></ui>`, `radius`},
		{"zero font size", testConfig, `<ui><text size="1,1" font-size="0"/></ui>`, `font-size`},
		{"infinite font size", testConfig, `<ui><text size="1,1" font-size="+Inf"/></ui>`, `font-size`},
		{"bad align", testConfig, `<ui><text size="1,1" align="middle"/></ui>`, `align`},
		{"bad valign", testConfig, `<ui><text size="1,1" valign="center"/></ui>`, `valign`},
		{"bad boolean", testConfig, `<ui><block size="1,1" visible="1"/></ui>`, `visible`},
		{"bad wrap", testConfig, `<ui><text size="1,1" wrap="TRUE"/></ui>`, `wrap`},
		{"malformed XML", testConfig, "<ui>\n  <block size=\"1,1\"></ui>", `invalid XML`},
		{"duplicate attribute", testConfig, `<ui color="#000000" color="#FFFFFF"/>`, `duplicate attribute`},
		{"multiple roots", testConfig, `<ui/><ui/>`, `exactly one root`},
		{"trailing text", testConfig, `<ui/>extra`, `outside the root`},
		{"namespace", testConfig, `<ui xmlns="example"/>`, `namespaces`},
		{"DTD", testConfig, `<!DOCTYPE ui><ui/>`, `directives`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fixture(t, tt.config, tt.main)
			_, err := Load(dir)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want error containing %q", err, tt.want)
			}
			var located *Error
			if !errors.As(err, &located) || located.Source.Line < 1 || located.Source.Column < 1 || !strings.HasPrefix(located.Source.File, dir) {
				t.Fatalf("error has no useful source location: %v", err)
			}
		})
	}
}

func TestWindowConfigOptions(t *testing.T) {
	config := `<config><window decorated="false" resizable="true" min-width="640" min-height="480"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`
	doc, err := Load(fixture(t, config, `<ui/>`))
	if err != nil {
		t.Fatal(err)
	}
	want := WindowConfig{
		Backend: "desktop", Width: 960, Height: 640,
		Decorated: false, Resizable: true, MinWidth: 640, MinHeight: 480,
	}
	if doc.Window != want {
		t.Fatalf("window config = %#v, want %#v", doc.Window, want)
	}
}

func TestRequiredFiles(t *testing.T) {
	for _, name := range []string{"config.xml", "main.xml"} {
		t.Run(name, func(t *testing.T) {
			dir := fixture(t, testConfig, `<ui/>`)
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			_, err := Load(dir)
			var located *Error
			if !errors.As(err, &located) || filepath.Base(located.Source.File) != name || !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing %s did not produce a located file error: %v", name, err)
			}
		})
	}
}

func TestUTF8BOMAndCDATA(t *testing.T) {
	doc, err := Load(fixture(t, "\ufeff"+testConfig, "\ufeff"+`<ui><text size="20,20"><![CDATA[A < B & C]]></text></ui>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Root.Children[0].Text.Content; got != "A < B & C" {
		t.Fatalf("CDATA content = %q", got)
	}
}

func TestProjectUI(t *testing.T) {
	doc, err := Load(filepath.Join("..", "..", "example", "hello-world", "ui"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Root == nil || doc.Fonts["default"] == nil {
		t.Fatal("the existing UI must remain loadable")
	}
}

func TestLoadFSIncludesApplicationSettings(t *testing.T) {
	files := fstest.MapFS{
		"config.xml":         &fstest.MapFile{Data: []byte(`<config><app name="Demo" version="2.3.4.5"/><fonts><font name="default" src="fonts/default.ttf"/></fonts></config>`), Mode: fs.ModePerm},
		"main.xml":           &fstest.MapFile{Data: []byte(`<ui><component src="component/card.xml" name="sample"/></ui>`), Mode: fs.ModePerm},
		"component/card.xml": &fstest.MapFile{Data: []byte(`<component><block id="panel" size="10,10"/></component>`), Mode: fs.ModePerm},
		"fonts/default.ttf":  &fstest.MapFile{Data: goregular.TTF, Mode: fs.ModePerm},
	}
	doc, err := LoadFS(files)
	if err != nil {
		t.Fatal(err)
	}
	if doc.AppName != "Demo" || doc.Version != "2.3.4.5" || doc.Find("sample.panel") == nil {
		t.Fatalf("embedded settings or component were not loaded: app=%q version=%q", doc.AppName, doc.Version)
	}
	if len(doc.Resources) != 2 || doc.Resources[0] != "fonts/default.ttf" || doc.Resources[1] != "component/card.xml" {
		t.Fatalf("unexpected collected resource paths: %#v", doc.Resources)
	}
}

func TestLoadFSRejectsRepeatedAppAndInvalidVersion(t *testing.T) {
	for _, config := range []string{
		`<config><app name="A" version="1.0.0"/><app name="B" version="1.0.0"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`,
		`<config><app name="A" version="1.0"/><fonts><font name="default" src="fonts/regular.ttf"/></fonts></config>`,
	} {
		directory := fixture(t, config, `<ui/>`)
		if _, err := Load(directory); err == nil {
			t.Fatalf("Load accepted invalid app config %q", config)
		}
	}
}

func TestComponentExpansionAndScopedIDs(t *testing.T) {
	dir := fixture(t, testConfig, `<ui>
  <component src="components/card.xml" name="left"/>
  <component src="components/card.xml" name="right"/>
</ui>`)
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("components/card.xml", `<component>
  <block id="panel" size="100,80">
    <text id=" title " size="90,24">Card</text>
    <component src="parts/status.xml" name="status"/>
  </block>
</component>`)
	write("components/parts/status.xml", `<component>
  <text id="value" position="0,30" size="90,24">Ready</text>
</component>`)

	doc, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"left.panel", "left.title", "left.status.value", "right.panel", "right.title", "right.status.value"} {
		if doc.Find(id) == nil {
			t.Errorf("expanded component is missing id %q", id)
		}
	}
	if len(doc.Root.Children) != 2 || doc.Find("left.title").Text.Content != "Card" {
		t.Fatal("component wrappers should disappear while retaining their children")
	}
}

func TestComponentErrors(t *testing.T) {
	tests := []struct {
		name  string
		main  string
		files map[string]string
		want  string
	}{
		{"duplicate instance", `<ui><component src="card.xml" name="same"/><component src="card.xml" name="same"/></ui>`, map[string]string{"card.xml": `<component><block size="1,1"/></component>`}, `duplicate component name`},
		{"cycle", `<ui><component src="a.xml" name="one"/></ui>`, map[string]string{"a.xml": `<component><component src="b.xml" name="two"/></component>`, "b.xml": `<component><component src="a.xml" name="three"/></component>`}, `cycle detected`},
		{"wrong root", `<ui><component src="card.xml" name="one"/></ui>`, map[string]string{"card.xml": `<block size="1,1"/>`}, `must have a <component> root`},
		{"empty component id", `<ui><component src="card.xml" name="one"/></ui>`, map[string]string{"card.xml": `<component><block id="" size="1,1"/></component>`}, `id must not be empty`},
		{"blank component id", `<ui><component src="card.xml" name="one"/></ui>`, map[string]string{"card.xml": `<component><block id="   " size="1,1"/></component>`}, `id must not be empty`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fixture(t, testConfig, tt.main)
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Load(dir)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want error containing %q", err, tt.want)
			}
			var located *Error
			if !errors.As(err, &located) || located.Source.File == "" || located.Source.Line < 1 || located.Source.Column < 1 {
				t.Fatalf("组件错误缺少文件和行列位置：%v", err)
			}
		})
	}
}

func TestImageAndButtonAttributes(t *testing.T) {
	dir := fixture(t, testConfig, `<ui>
  <image id="photo" size="16,8" src="assets/photo.png" fit="contain"/>
  <button id="run" size="20,10" disabled="true" hover-color="#2A5C80"/>
</ui>`)
	path := filepath.Join(dir, "assets", "photo.png")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	photo, button := doc.Find("photo"), doc.Find("run")
	if photo.Image == nil || photo.Image.Image.Bounds().Size() != image.Pt(4, 2) || photo.Image.Fit != "contain" {
		t.Fatalf("image attributes were not loaded: %#v", photo.Image)
	}
	if button.Button == nil || !button.Button.Disabled || button.Button.HoverColor == nil || *button.Button.HoverColor != (color.NRGBA{R: 42, G: 92, B: 128, A: 255}) {
		t.Fatalf("button attributes were not loaded: %#v", button.Button)
	}
}
