// Package software 使用 CPU 将 GoUI 文档绘制到 RGBA 帧缓冲。
package software

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/lwy183178053/GoUI/internal/layout"
	"github.com/lwy183178053/GoUI/internal/ui"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type faceKey struct {
	name string
	size float64
}

// Renderer 管理字体和缓存帧，所有方法应串行调用。
type Renderer struct {
	document *ui.Document
	faces    map[faceKey]font.Face
	layout   *layout.Box
	boxes    map[string]*layout.Box
	nodes    map[string]*ui.Node
	frame    *image.RGBA
	dirty    image.Rectangle
	handlers map[string]map[ui.ButtonEvent]func() error
	motions  map[string]*animation
	world    map[*ui.Node]affine
	hovered  *ui.Node
	pressed  *ui.Node
	closed   bool
}

func New(document *ui.Document) (*Renderer, error) {
	if document == nil || document.Root == nil {
		return nil, errors.New("cannot render an empty document")
	}
	renderer := &Renderer{
		document: document,
		faces:    make(map[faceKey]font.Face),
		boxes:    make(map[string]*layout.Box),
		nodes:    make(map[string]*ui.Node),
		handlers: make(map[string]map[ui.ButtonEvent]func() error),
		motions:  make(map[string]*animation),
		world:    make(map[*ui.Node]affine),
	}
	renderer.indexNodes(document.Root)
	if err := renderer.prepare(document.Root); err != nil {
		_ = renderer.Close()
		return nil, err
	}
	return renderer, nil
}

func (r *Renderer) prepare(node *ui.Node) error {
	if node.Text != nil {
		key := faceKey{node.Text.Font, node.Text.FontSize}
		if _, exists := r.faces[key]; !exists {
			parsed := r.document.Fonts[key.name]
			if parsed == nil {
				return fmt.Errorf("unregistered font %q", key.name)
			}
			face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: key.size, DPI: 72, Hinting: font.HintingFull})
			if err != nil {
				return &ui.Error{Source: node.Source, Err: fmt.Errorf("create font face: %w", err)}
			}
			r.faces[key] = face
		}
	}
	for _, child := range node.Children {
		if err := r.prepare(child); err != nil {
			return err
		}
	}
	return nil
}

// Frame 实现 backend.FrameSource。尺寸不变时复用已绘制的画面。
func (r *Renderer) Frame(width, height int) (*image.RGBA, error) {
	if r.closed {
		return nil, errors.New("renderer is closed")
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("render size must be positive, got %dx%d", width, height)
	}
	if r.layout == nil || r.frame.Rect.Dx() != width || r.frame.Rect.Dy() != height {
		r.frame = image.NewRGBA(image.Rect(0, 0, width, height))
		r.rebuildLayout(width, height)
		r.dirty = r.frame.Bounds()
	}
	if r.advanceAnimations(time.Now()) {
		r.invalidate(r.frame.Bounds())
	}
	if r.dirty.Empty() {
		return r.frame, nil
	}
	next := image.NewRGBA(r.frame.Bounds())
	draw.Draw(next, next.Bounds(), r.frame, r.frame.Bounds().Min, draw.Src)
	dirty := r.dirty.Intersect(next.Bounds())
	draw.Draw(next, dirty, image.Transparent, image.Point{}, draw.Src)
	r.world = make(map[*ui.Node]affine)
	r.paint(next, r.layout, dirty, identity)
	r.frame = next
	r.dirty = image.Rectangle{}
	return r.frame, nil
}

func (r *Renderer) rebuildLayout(width, height int) {
	r.layout = layout.Compute(r.document.Root, width, height)
	r.boxes = make(map[string]*layout.Box)
	r.indexBoxes(r.layout)
}

func (r *Renderer) indexBoxes(box *layout.Box) {
	if box.Node.ID != "" {
		r.boxes[box.Node.ID] = box
	}
	for _, child := range box.Children {
		r.indexBoxes(child)
	}
}

func (r *Renderer) indexNodes(node *ui.Node) {
	if node.ID != "" {
		r.nodes[node.ID] = node
	}
	for _, child := range node.Children {
		r.indexNodes(child)
	}
}

func (r *Renderer) paint(frame *image.RGBA, box *layout.Box, limit image.Rectangle, parent affine) {
	node := box.Node
	if !node.Visible {
		return
	}
	transform := multiply(parent, nodeTransform(box, r.motions[node.ID]))
	r.world[node] = transform
	if transform == identity {
		r.paintSelf(frame, box, limit)
	} else if !box.Pixels.Empty() {
		layer := image.NewRGBA(box.Pixels)
		r.paintSelf(layer, box, layer.Bounds())
		xdraw.CatmullRom.Transform(frame, toAff3(transform), layer, layer.Bounds(), draw.Over, nil)
	}
	// 子节点按同一组父级变换绘制，但不受父矩形裁剪。
	for _, child := range box.Children {
		r.paint(frame, child, limit, transform)
	}
}

func (r *Renderer) paintSelf(frame *image.RGBA, box *layout.Box, limit image.Rectangle) {
	node := box.Node
	if node.Kind == ui.UI {
		clip := box.Pixels.Intersect(frame.Bounds()).Intersect(limit)
		if !clip.Empty() {
			draw.Draw(frame, clip, image.NewUniform(node.Color), image.Point{}, draw.Src)
		}
	} else {
		fillRoundedRectClip(frame, box.Pixels, node.Color, node.Radius, limit)
	}
	if node.Image != nil {
		paintImage(frame, box.Pixels, limit, node.Image, node.Radius)
	}
	if node.Text != nil {
		face := r.faces[faceKey{node.Text.Font, node.Text.FontSize}]
		paintText(frame, box.Pixels, node.Text, face, limit)
	}
}

func (r *Renderer) node(id string) (*ui.Node, *layout.Box, error) {
	if r.closed {
		return nil, nil, errors.New("renderer is closed")
	}
	node := r.nodes[id]
	if node == nil {
		return nil, nil, fmt.Errorf("unknown UI id %q", id)
	}
	return node, r.boxes[id], nil
}

// Kind 返回节点标签，用于公开组件 Getter 的类型校验。
func (r *Renderer) Kind(id string) (ui.Kind, error) {
	node, _, err := r.node(id)
	if err != nil {
		return "", err
	}
	return node.Kind, nil
}

func (r *Renderer) invalidate(rect image.Rectangle) {
	if r.frame == nil {
		return
	}
	rect = rect.Intersect(r.frame.Bounds())
	if rect.Empty() {
		return
	}
	for id, motion := range r.motions {
		if motion.move != nil || motion.position != [2]float64{float64(r.nodes[id].Position.X), float64(r.nodes[id].Position.Y)} ||
			motion.rotation != 0 || motion.scale != [2]float64{1, 1} {
			r.dirty = r.frame.Bounds()
			return
		}
	}
	if r.dirty.Empty() {
		r.dirty = rect
	} else {
		r.dirty = r.dirty.Union(rect)
	}
}

// SetText 修改 Text 内容，并重绘它所在的矩形。
func (r *Renderer) SetText(id, value string) error {
	node, box, err := r.node(id)
	if err != nil {
		return err
	}
	if node.Text == nil {
		return fmt.Errorf("UI id %q is not a text node", id)
	}
	if node.Text.Content != value {
		node.Text.Content = value
		if box != nil {
			r.invalidate(box.Pixels)
		}
	}
	return nil
}

// SetColor 修改节点自身的填充色，不影响子节点。
func (r *Renderer) SetColor(id string, value color.NRGBA) error {
	node, box, err := r.node(id)
	if err != nil {
		return err
	}
	if node.Color != value {
		node.Color = value
		if box != nil {
			r.invalidate(box.Pixels)
		}
	}
	return nil
}

// SetTextColor 修改文字颜色，不改变文字下方的填充色。
func (r *Renderer) SetTextColor(id string, value color.NRGBA) error {
	node, box, err := r.node(id)
	if err != nil {
		return err
	}
	if node.Text == nil {
		return fmt.Errorf("UI id %q is not a text node", id)
	}
	if node.Text.Color != value {
		node.Text.Color = value
		if box != nil {
			r.invalidate(box.Pixels)
		}
	}
	return nil
}

// SetVisible 显示或隐藏节点及其整个子树。
func (r *Renderer) SetVisible(id string, visible bool) error {
	node, box, err := r.node(id)
	if err != nil {
		return err
	}
	if node.Visible != visible {
		node.Visible = visible
		if box != nil {
			r.invalidate(layout.SubtreeBounds(box))
		}
	}
	return nil
}

// SetDisabled 修改按钮是否响应点击。
func (r *Renderer) SetDisabled(id string, disabled bool) error {
	node, box, err := r.node(id)
	if err != nil {
		return err
	}
	if node.Button == nil {
		return fmt.Errorf("UI id %q is not a button", id)
	}
	if node.Button.Disabled != disabled {
		node.Button.Disabled = disabled
		if box != nil {
			r.invalidate(layout.SubtreeBounds(box))
		}
	}
	return nil
}

func (r *Renderer) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var errs []error
	for _, face := range r.faces {
		if err := face.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	r.faces = nil
	r.frame = nil
	r.layout = nil
	r.boxes = nil
	r.nodes = nil
	r.handlers = nil
	r.motions = nil
	r.world = nil
	return errors.Join(errs...)
}
