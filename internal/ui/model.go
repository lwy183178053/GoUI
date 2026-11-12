// Package ui 负责加载与平台无关的 GoUI 文档。
package ui

import (
	"fmt"
	"image"
	"image/color"

	"golang.org/x/image/font/opentype"
)

// Source 记录节点或错误在 XML 中的位置。
type Source struct {
	File   string
	Line   int
	Column int
}

// Error 是带 XML 文件位置的加载错误。
type Error struct {
	Source Source
	Err    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %v", e.Source.File, e.Source.Line, e.Source.Column, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func at(source Source, format string, args ...any) error {
	return &Error{Source: source, Err: fmt.Errorf(format, args...)}
}

type Kind string

type ButtonEvent uint8

const (
	UI     Kind = "ui"
	Block  Kind = "block"
	Text   Kind = "text"
	Image  Kind = "image"
	Button Kind = "button"
)

const (
	MouseEnter ButtonEvent = iota
	MouseLeave
	MouseDown
	MouseUp
	Click
)

type Vec2 struct{ X, Y float64 }
type Insets struct{ Left, Top, Right, Bottom int }

// BlockStyle 是所有可视节点共用的属性；颜色使用非预乘 Alpha。
type BlockStyle struct {
	ID       string
	Visible  bool
	Anchor   Vec2
	Pivot    Vec2
	Position image.Point
	Size     image.Point
	Margin   Insets
	Color    color.NRGBA
	Radius   float64
}

type TextStyle struct {
	Content  string
	Font     string
	FontSize float64
	Color    color.NRGBA
	Align    string
	VAlign   string
	Wrap     bool
}

type ImageStyle struct {
	Source string
	Image  image.Image
	Fit    string
}

type ButtonStyle struct {
	Disabled   bool
	HoverColor *color.NRGBA
}

type WindowConfig struct {
	Backend    string
	Display    int
	Width      int
	Height     int
	Fullscreen bool
	Decorated  bool
	Resizable  bool
	MinWidth   int
	MinHeight  int
}

type Node struct {
	Kind Kind
	BlockStyle
	Text     *TextStyle
	Image    *ImageStyle
	Button   *ButtonStyle
	Children []*Node
	Source   Source
}

// Document 保存已加载的界面和字体。节点树交给 Renderer 管理，更新时使用 Renderer 的方法。
type Document struct {
	Directory   string
	AppName     string
	Version     string
	Window      WindowConfig
	Resources   []string
	Root        *Node
	Fonts       map[string]*opentype.Font
	nodes       map[string]*Node
	resourceSet map[string]struct{}
}

func (d *Document) Find(id string) *Node { return d.nodes[id] }

func (d *Document) addResource(path string) {
	if d.resourceSet == nil {
		d.resourceSet = make(map[string]struct{})
	}
	if _, exists := d.resourceSet[path]; exists {
		return
	}
	d.resourceSet[path] = struct{}{}
	d.Resources = append(d.Resources, path)
}
