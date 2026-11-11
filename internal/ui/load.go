package ui

import (
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
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
	doc := &Document{Directory: directory, nodes: make(map[string]*Node)}
	config, err := readXML(filepath.Join(directory, "config.xml"))
	if err != nil {
		return nil, err
	}
	doc.Fonts, err = loadFonts(config, directory)
	if err != nil {
		return nil, err
	}
	main, err := readXML(filepath.Join(directory, "main.xml"))
	if err != nil {
		return nil, err
	}
	if main.name != "ui" {
		return nil, at(main.source, "main.xml must have a <ui> root")
	}
	mainPath, err := filepath.Abs(filepath.Join(directory, "main.xml"))
	if err != nil {
		return nil, at(main.source, "resolve main.xml path: %w", err)
	}
	main.children, err = expandChildren(main.children, filepath.Dir(mainPath), "", map[string]bool{mainPath: true}, make(map[string]bool), directory)
	if err != nil {
		return nil, err
	}
	doc.Root, err = loadNode(main, doc, true)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func loadFonts(config *element, directory string) (map[string]*opentype.Font, error) {
	if config.name != "config" {
		return nil, at(config.source, "config.xml must have a <config> root")
	}
	if err := config.attributes(); err != nil {
		return nil, err
	}
	if err := config.noText(); err != nil {
		return nil, err
	}
	if len(config.children) != 1 || config.children[0].name != "fonts" {
		return nil, at(config.source, "<config> requires exactly one <fonts> element")
	}
	fonts := config.children[0]
	if err := fonts.attributes(); err != nil {
		return nil, err
	}
	if err := fonts.noText(); err != nil {
		return nil, err
	}
	result := make(map[string]*opentype.Font)
	for _, entry := range fonts.children {
		if entry.name != "font" {
			return nil, at(entry.source, "unsupported tag <%s> in <fonts>", entry.name)
		}
		if err := entry.attributes("name", "src"); err != nil {
			return nil, err
		}
		if len(entry.children) != 0 {
			return nil, at(entry.source, "<font> cannot contain child elements")
		}
		if err := entry.noText(); err != nil {
			return nil, err
		}
		name, err := entry.required("name")
		if err != nil {
			return nil, err
		}
		if _, exists := result[name]; exists {
			return nil, at(entry.source, "duplicate font name %q", name)
		}
		src, err := entry.required("src")
		if err != nil {
			return nil, err
		}
		path, pathErr := assetPath(directory, src)
		if pathErr != nil {
			return nil, at(entry.source, "font src must be relative to the UI directory")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, at(entry.source, "read font %q: %w", name, err)
		}
		parsed, err := opentype.Parse(data)
		if err != nil {
			return nil, at(entry.source, "parse font %q (%s): %w", name, src, err)
		}
		result[name] = parsed
	}
	if _, exists := result["default"]; !exists {
		return nil, at(fonts.source, "<fonts> must register a font named %q", "default")
	}
	return result, nil
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
		allowed = append(append([]string(nil), commonAttributes...), "src", "fit")
		node.Image = &ImageStyle{Fit: "stretch"}
	case node.Kind == Button:
		allowed = append(append([]string(nil), commonAttributes...), "on-click", "disabled")
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
		if err := loadImage(e, node.Image, doc.Directory); err != nil {
			return nil, err
		}
	} else {
		if err := e.noText(); err != nil {
			return nil, err
		}
		for _, child := range e.children {
			loaded, err := loadNode(child, doc, false)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, loaded)
		}
	}
	return node, nil
}

func loadImage(e *element, style *ImageStyle, directory string) error {
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
	path, err := assetPath(directory, src)
	if err != nil {
		return at(e.source, "image src must be relative to the UI directory")
	}
	file, err := os.Open(path)
	if err != nil {
		return at(e.source, "read image %q: %w", src, err)
	}
	decoded, _, decodeErr := image.Decode(file)
	closeErr := file.Close()
	if decodeErr != nil {
		return at(e.source, "decode image %q: %w", src, decodeErr)
	}
	if decoded.Bounds().Empty() {
		return at(e.source, "image %q has no pixels", src)
	}
	if closeErr != nil {
		return at(e.source, "close image %q: %w", src, closeErr)
	}
	style.Source, style.Image = src, decoded
	return nil
}

func assetPath(directory, src string) (string, error) {
	src = filepath.FromSlash(src)
	if filepath.IsAbs(src) || filepath.VolumeName(src) != "" || strings.HasPrefix(src, `\`) {
		return "", os.ErrInvalid
	}
	path, err := filepath.Abs(filepath.Join(directory, src))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(directory, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", os.ErrInvalid
	}
	return path, nil
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
		if value, exists := e.attrs["on-click"]; exists {
			node.Button.Action = strings.TrimSpace(value)
			if node.Button.Action == "" {
				return at(e.source, "on-click must not be empty")
			}
		}
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
