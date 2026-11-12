package ui

import (
	"bytes"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/image/font/opentype"
)

// Load 读取 config.xml 和 main.xml，加载字体并校验文档。
// 字体只从 UI 目录读取，不查询系统字体。
func Load(directory string) (*Document, error) {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, at(Source{directory, 1, 1}, "resolve UI directory: %w", err)
	}
	doc, err := loadFS(os.DirFS(directory), directory)
	if err != nil {
		return nil, err
	}
	doc.Directory = directory
	return doc, nil
}

// LoadFS 从文件系统根目录加载 config.xml、main.xml 和引用的资源。
func LoadFS(fsys fs.FS) (*Document, error) {
	return loadFS(fsys, "")
}

func loadFS(fsys fs.FS, sourceRoot string) (*Document, error) {
	if fsys == nil {
		return nil, at(Source{".", 1, 1}, "UI file system is required")
	}
	doc := &Document{
		Version:     "0.0.0",
		Window:      WindowConfig{Backend: "desktop", Width: 960, Height: 640},
		nodes:       make(map[string]*Node),
		resourceSet: make(map[string]struct{}),
	}
	config, err := readXML(fsys, "config.xml", sourceName(sourceRoot, "config.xml"))
	if err != nil {
		return nil, err
	}
	doc.Fonts, doc.AppName, doc.Version, doc.Window, err = loadConfig(config, fsys, doc)
	if err != nil {
		return nil, err
	}
	main, err := readXML(fsys, "main.xml", sourceName(sourceRoot, "main.xml"))
	if err != nil {
		return nil, err
	}
	if main.name != "ui" {
		return nil, at(main.source, "main.xml must have a <ui> root")
	}
	main.children, err = expandChildren(fsys, main.children, "main.xml", "", map[string]bool{"main.xml": true}, make(map[string]bool), sourceRoot, doc)
	if err != nil {
		return nil, err
	}
	doc.Root, err = loadNode(main, doc, true, fsys)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func sourceName(root, filename string) string {
	if root == "" {
		return filename
	}
	return filepath.Join(root, filepath.FromSlash(filename))
}

type appSettings struct {
	name    string
	version string
}

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?$`)

func loadConfig(config *element, fsys fs.FS, doc *Document) (map[string]*opentype.Font, string, string, WindowConfig, error) {
	if config.name != "config" {
		return nil, "", "", WindowConfig{}, at(config.source, "config.xml must have a <config> root")
	}
	if err := config.attributes(); err != nil {
		return nil, "", "", WindowConfig{}, err
	}
	if err := config.noText(); err != nil {
		return nil, "", "", WindowConfig{}, err
	}
	var fonts *element
	var app *appSettings
	window := doc.Window
	windowSet := false
	for _, child := range config.children {
		switch child.name {
		case "fonts":
			if fonts != nil {
				return nil, "", "", WindowConfig{}, at(child.source, "<config> can contain only one <fonts> element")
			}
			fonts = child
		case "app":
			if app != nil {
				return nil, "", "", WindowConfig{}, at(child.source, "<config> can contain only one <app> element")
			}
			if err := child.attributes("name", "version"); err != nil {
				return nil, "", "", WindowConfig{}, err
			}
			if len(child.children) != 0 {
				return nil, "", "", WindowConfig{}, at(child.source, "<app> cannot contain child elements")
			}
			if err := child.noText(); err != nil {
				return nil, "", "", WindowConfig{}, err
			}
			name, err := child.required("name")
			if err != nil {
				return nil, "", "", WindowConfig{}, err
			}
			version, err := child.required("version")
			if err != nil {
				return nil, "", "", WindowConfig{}, err
			}
			if !versionPattern.MatchString(version) {
				return nil, "", "", WindowConfig{}, at(child.source, "app version must be major.minor.patch or major.minor.patch.build")
			}
			app = &appSettings{name: name, version: version}
		case "window":
			if windowSet {
				return nil, "", "", WindowConfig{}, at(child.source, "<config> can contain only one <window> element")
			}
			windowSet = true
			parsed, err := loadWindow(child, doc.Window)
			if err != nil {
				return nil, "", "", WindowConfig{}, err
			}
			window = parsed
		default:
			return nil, "", "", WindowConfig{}, at(child.source, "unsupported tag <%s> in <config>; config.xml requires <fonts>", child.name)
		}
	}
	if fonts == nil {
		return nil, "", "", WindowConfig{}, at(config.source, "<config> requires exactly one <fonts> element")
	}
	if err := fonts.attributes(); err != nil {
		return nil, "", "", WindowConfig{}, err
	}
	if err := fonts.noText(); err != nil {
		return nil, "", "", WindowConfig{}, err
	}
	result := make(map[string]*opentype.Font)
	for _, entry := range fonts.children {
		if entry.name != "font" {
			return nil, "", "", WindowConfig{}, at(entry.source, "unsupported tag <%s> in <fonts>", entry.name)
		}
		if err := entry.attributes("name", "src"); err != nil {
			return nil, "", "", WindowConfig{}, err
		}
		if len(entry.children) != 0 {
			return nil, "", "", WindowConfig{}, at(entry.source, "<font> cannot contain child elements")
		}
		if err := entry.noText(); err != nil {
			return nil, "", "", WindowConfig{}, err
		}
		name, err := entry.required("name")
		if err != nil {
			return nil, "", "", WindowConfig{}, err
		}
		if _, exists := result[name]; exists {
			return nil, "", "", WindowConfig{}, at(entry.source, "duplicate font name %q", name)
		}
		src, err := entry.required("src")
		if err != nil {
			return nil, "", "", WindowConfig{}, err
		}
		asset, pathErr := assetPath(src)
		if pathErr != nil {
			return nil, "", "", WindowConfig{}, at(entry.source, "font src must be relative to the UI directory")
		}
		data, err := fs.ReadFile(fsys, asset)
		if err != nil {
			return nil, "", "", WindowConfig{}, at(entry.source, "read font %q: %w", name, err)
		}
		doc.addResource(asset)
		parsed, err := opentype.Parse(data)
		if err != nil {
			return nil, "", "", WindowConfig{}, at(entry.source, "parse font %q (%s): %w", name, src, err)
		}
		result[name] = parsed
	}
	if _, exists := result["default"]; !exists {
		return nil, "", "", WindowConfig{}, at(fonts.source, "<fonts> must register a font named %q", "default")
	}
	if app == nil {
		return result, "", "0.0.0", window, nil
	}
	return result, app.name, app.version, window, nil
}

func loadWindow(e *element, defaults WindowConfig) (WindowConfig, error) {
	if err := e.attributes("backend", "display", "width", "height", "fullscreen"); err != nil {
		return WindowConfig{}, err
	}
	if len(e.children) != 0 {
		return WindowConfig{}, at(e.source, "<window> cannot contain child elements")
	}
	if err := e.noText(); err != nil {
		return WindowConfig{}, err
	}
	if value, exists := e.attrs["backend"]; exists {
		if value != "desktop" && value != "drm" {
			return WindowConfig{}, at(e.source, "window backend must be desktop or drm")
		}
		defaults.Backend = value
	}
	for _, item := range []struct {
		name   string
		target *int
		min    int
	}{{"display", &defaults.Display, 0}, {"width", &defaults.Width, 1}, {"height", &defaults.Height, 1}} {
		if value, exists := e.attrs[item.name]; exists {
			number, err := strconv.Atoi(value)
			if err != nil || number < item.min {
				return WindowConfig{}, at(e.source, "window %s must be an integer >= %d", item.name, item.min)
			}
			*item.target = number
		}
	}
	if value, exists := e.attrs["fullscreen"]; exists {
		fullscreen, err := boolean(value)
		if err != nil {
			return WindowConfig{}, at(e.source, "window fullscreen must be true or false")
		}
		defaults.Fullscreen = fullscreen
	}
	return defaults, nil
}

var commonAttributes = []string{
	"id", "visible", "anchor", "pivot", "position", "size", "margin", "color", "radius",
}

func loadNode(e *element, doc *Document, root bool, fsys fs.FS) (*Node, error) {
	node := &Node{Kind: Kind(e.name), Source: e.source, BlockStyle: BlockStyle{Visible: true}}
	allowed := commonAttributes
	switch {
	case root:
		allowed = []string{"id", "color"}
		node.Color = color.NRGBA{A: 255}
	case node.Kind == Block:
	case node.Kind == Text:
		allowed = append(append([]string(nil), commonAttributes...), "font", "font-size", "text-color", "align", "valign", "wrap")
		node.Text = &TextStyle{Font: "default", FontSize: 16, Color: color.NRGBA{R: 244, G: 247, B: 247, A: 255}, Align: "left", VAlign: "middle"}
	case node.Kind == Image:
		allowed = append(append([]string(nil), commonAttributes...), "src", "fit")
		node.Image = &ImageStyle{Fit: "stretch"}
	case node.Kind == Button:
		allowed = append(append([]string(nil), commonAttributes...), "disabled")
		node.Button = &ButtonStyle{}
	default:
		return nil, at(e.source, "unsupported tag <%s>", e.name)
	}
	if err := e.attributes(allowed...); err != nil {
		return nil, err
	}
	if value, exists := e.attrs["id"]; exists {
		id := strings.TrimSpace(value)
		if id == "" {
			return nil, at(e.source, "id must not be empty")
		}
		if previous, exists := doc.nodes[id]; exists {
			return nil, at(e.source, "duplicate id %q (first defined at %s:%d:%d)", id, previous.Source.File, previous.Source.Line, previous.Source.Column)
		}
		node.ID = id
		doc.nodes[id] = node
	}
	if value, exists := e.attrs["color"]; exists {
		parsed, err := parseColor(value)
		if err != nil {
			return nil, at(e.source, "color must be #RRGGBB or #RRGGBBAA")
		}
		node.Color = parsed
	}
	if !root {
		if err := loadBlock(e, node); err != nil {
			return nil, err
		}
	}
	if node.Kind == Text {
		if len(e.children) != 0 {
			return nil, at(e.children[0].source, "<text> may contain text only, not child elements")
		}
		if err := loadText(e, node.Text, doc); err != nil {
			return nil, err
		}
	} else if node.Kind == Image {
		if len(e.children) != 0 {
			return nil, at(e.children[0].source, "<image> cannot contain child elements")
		}
		if err := e.noText(); err != nil {
			return nil, err
		}
		if err := loadImage(e, node.Image, fsys, doc); err != nil {
			return nil, err
		}
	} else {
		if err := e.noText(); err != nil {
			return nil, err
		}
		for _, child := range e.children {
			loaded, err := loadNode(child, doc, false, fsys)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, loaded)
		}
	}
	return node, nil
}

func loadImage(e *element, style *ImageStyle, fsys fs.FS, doc *Document) error {
	src, err := e.required("src")
	if err != nil {
		return err
	}
	if value, exists := e.attrs["fit"]; exists {
		if value != "stretch" && value != "contain" && value != "cover" {
			return at(e.source, "fit must be stretch, contain or cover")
		}
		style.Fit = value
	}
	asset, err := assetPath(src)
	if err != nil {
		return at(e.source, "image src must be relative to the UI directory")
	}
	data, err := fs.ReadFile(fsys, asset)
	if err != nil {
		return at(e.source, "read image %q: %w", src, err)
	}
	decoded, _, decodeErr := image.Decode(bytes.NewReader(data))
	if decodeErr != nil {
		return at(e.source, "decode image %q: %w", src, decodeErr)
	}
	if decoded.Bounds().Empty() {
		return at(e.source, "image %q has no pixels", src)
	}
	style.Source, style.Image = src, decoded
	doc.addResource(asset)
	return nil
}

func assetPath(src string) (string, error) {
	if src == "" || strings.Contains(src, `\`) || path.IsAbs(src) || strings.Contains(src, ":") {
		return "", fs.ErrInvalid
	}
	clean := path.Clean(src)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || !fs.ValidPath(clean) {
		return "", fs.ErrInvalid
	}
	return clean, nil
}

func loadBlock(e *element, node *Node) error {
	size, err := e.required("size")
	if err != nil {
		return err
	}
	values, err := integers(size, 2)
	if err != nil || values[0] < -1 || values[1] < -1 {
		return at(e.source, "size must contain two integers >= -1")
	}
	node.Size = image.Pt(values[0], values[1])
	for _, attribute := range []struct {
		name   string
		target *Vec2
	}{{"anchor", &node.Anchor}, {"pivot", &node.Pivot}} {
		if value, exists := e.attrs[attribute.name]; exists {
			parsed, err := ratio(value)
			if err != nil {
				return at(e.source, "%s must contain two finite numbers between 0 and 1", attribute.name)
			}
			*attribute.target = parsed
		}
	}
	if value, exists := e.attrs["position"]; exists {
		values, err := integers(value, 2)
		if err != nil {
			return at(e.source, "position must contain two integer pixels")
		}
		node.Position = image.Pt(values[0], values[1])
	}
	if value, exists := e.attrs["margin"]; exists {
		values, err := integers(value, 4)
		if err != nil {
			return at(e.source, "margin must contain four non-negative integer pixels: left,top,right,bottom")
		}
		for _, n := range values {
			if n < 0 {
				return at(e.source, "margin values must be non-negative")
			}
		}
		node.Margin = Insets{values[0], values[1], values[2], values[3]}
	}
	if value, exists := e.attrs["visible"]; exists {
		parsed, err := boolean(value)
		if err != nil {
			return at(e.source, "visible must be true or false")
		}
		node.Visible = parsed
	}
	if value, exists := e.attrs["radius"]; exists {
		parsed, err := number(value)
		if err != nil || parsed < 0 {
			return at(e.source, "radius must be a finite non-negative pixel value")
		}
		node.Radius = parsed
	}
	if node.Button != nil {
		if value, exists := e.attrs["disabled"]; exists {
			parsed, err := boolean(value)
			if err != nil {
				return at(e.source, "disabled must be true or false")
			}
			node.Button.Disabled = parsed
		}
	}
	return nil
}

func loadText(e *element, text *TextStyle, doc *Document) error {
	text.Content = strings.TrimSpace(e.text.String())
	if value, exists := e.attrs["font"]; exists {
		text.Font = strings.TrimSpace(value)
	}
	if _, exists := doc.Fonts[text.Font]; !exists {
		return at(e.source, "text references unregistered font %q", text.Font)
	}
	if value, exists := e.attrs["font-size"]; exists {
		parsed, err := number(value)
		if err != nil || parsed <= 0 {
			return at(e.source, "font-size must be a finite positive pixel value")
		}
		text.FontSize = parsed
	}
	if value, exists := e.attrs["text-color"]; exists {
		parsed, err := parseColor(value)
		if err != nil {
			return at(e.source, "text-color must be #RRGGBB or #RRGGBBAA")
		}
		text.Color = parsed
	}
	if value, exists := e.attrs["align"]; exists {
		if value != "left" && value != "center" && value != "right" {
			return at(e.source, "align must be left, center or right")
		}
		text.Align = value
	}
	if value, exists := e.attrs["valign"]; exists {
		if value != "top" && value != "middle" && value != "bottom" {
			return at(e.source, "valign must be top, middle or bottom")
		}
		text.VAlign = value
	}
	if value, exists := e.attrs["wrap"]; exists {
		parsed, err := boolean(value)
		if err != nil {
			return at(e.source, "wrap must be true or false")
		}
		text.Wrap = parsed
	}
	return nil
}
