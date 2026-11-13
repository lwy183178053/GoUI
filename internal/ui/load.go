package ui

import (
	"image"
	"image/color"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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
		Version: "0.0.0",
		Window: WindowConfig{
			Backend: "desktop", Width: 960, Height: 640, Decorated: true,
		},
		files:       fsys,
		nodes:       make(map[string]*Node),
		resourceSet: make(map[string]struct{}),
	}
	config, err := readXML(fsys, "config.xml", sourceName(sourceRoot, "config.xml"))
	if err != nil {
		return nil, err
	}
	if err := loadConfig(config, fsys, doc); err != nil {
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
	doc.Root, err = loadNode(main, doc, true)
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

var commonAttributes = []string{
	"id", "visible", "anchor", "pivot", "position", "size", "margin", "color", "radius",
}

func loadNode(e *element, doc *Document, root bool) (*Node, error) {
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
		allowed = append(append([]string(nil), commonAttributes...), "src", "fit", "playback")
		node.Image = &ImageStyle{Fit: "stretch"}
	case node.Kind == Button:
		allowed = append(append([]string(nil), commonAttributes...), "disabled", "hover-color")
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
	if node.Button != nil {
		if value, exists := e.attrs["hover-color"]; exists {
			parsed, err := parseColor(value)
			if err != nil {
				return nil, at(e.source, "hover-color must be #RRGGBB or #RRGGBBAA")
			}
			node.Button.HoverColor = &parsed
		}
	}
	if node.Kind == Text {
		if len(e.children) != 0 {
			return nil, at(e.children[0].source, "<text> may contain text only, not child elements")
		}
		if err := loadText(e, node.Text, doc); err != nil {
			return nil, err
		}
		return node, nil
	}
	if err := e.noText(); err != nil {
		return nil, err
	}
	if node.Kind == Image {
		if err := loadImage(e, node.Image, doc); err != nil {
			return nil, err
		}
	}
	// Image 与 Block 一样提供子节点坐标空间，文字可直接叠加在图片上。
	for _, child := range e.children {
		loaded, err := loadNode(child, doc, false)
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, loaded)
	}
	return node, nil
}

func loadImage(e *element, style *ImageStyle, doc *Document) error {
	src, err := e.required("src")
	if err != nil {
		return err
	}
	fit := "stretch"
	if value, exists := e.attrs["fit"]; exists {
		if value != "stretch" && value != "contain" && value != "cover" {
			return at(e.source, "fit must be stretch, contain or cover")
		}
		fit = value
	}
	playback := "auto"
	if value, exists := e.attrs["playback"]; exists {
		if value != "auto" && value != "segmented" {
			return at(e.source, "playback must be auto or segmented")
		}
		playback = value
	}
	loaded, err := doc.LoadImage(src, playback, e.source)
	if err != nil {
		return err
	}
	*style = *loaded
	style.Fit = fit
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
