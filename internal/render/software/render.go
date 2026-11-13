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
	// 两个帧句柄共用像素内存，交替返回以通知窗口后端画面有更新。
	frameA       *image.RGBA
	frameB       *image.RGBA
	published    *image.RGBA
	dirty        image.Rectangle
	images       map[*ui.Node]*image.RGBA
	layers       map[*ui.Node]*image.RGBA
	maskCache    map[roundedMaskKey]*image.Alpha
	handlers     map[string]map[ui.ButtonEvent]func() error
	motions      map[string]*animation
	imagePlayers map[*ui.Node]*imagePlayback
	world        map[*ui.Node]affine
	hovered      *ui.Node
	pressed      *ui.Node
	closed       bool
}

func New(document *ui.Document) (*Renderer, error) {
	if document == nil || document.Root == nil {
		return nil, errors.New("cannot render an empty document")
	}
	renderer := &Renderer{
		document:     document,
		faces:        make(map[faceKey]font.Face),
		boxes:        make(map[string]*layout.Box),
		nodes:        make(map[string]*ui.Node),
		images:       make(map[*ui.Node]*image.RGBA),
		layers:       make(map[*ui.Node]*image.RGBA),
		maskCache:    make(map[roundedMaskKey]*image.Alpha),
		handlers:     make(map[string]map[ui.ButtonEvent]func() error),
		motions:      make(map[string]*animation),
		imagePlayers: make(map[*ui.Node]*imagePlayback),
		world:        make(map[*ui.Node]affine),
	}
	if err := renderer.prepare(document.Root); err != nil {
		_ = renderer.Close()
		return nil, err
	}
	return renderer, nil
}

func (r *Renderer) prepare(node *ui.Node) error {
	if node.ID != "" {
		r.nodes[node.ID] = node
	}
	if node.Image != nil && node.Image.Animation != nil {
		r.imagePlayers[node] = newImagePlayback(node.Image.Animation)
	}
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

// Frame 实现 backend.FrameSource。画面变化时重用像素内存，并切换帧句柄。
func (r *Renderer) Frame(width, height int) (*image.RGBA, error) {
	if r.closed {
		return nil, errors.New("renderer is closed")
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("render size must be positive, got %dx%d", width, height)
	}
	if r.layout == nil || r.frame.Rect.Dx() != width || r.frame.Rect.Dy() != height {
		r.frame = image.NewRGBA(image.Rect(0, 0, width, height))
		r.frameA = &image.RGBA{Pix: r.frame.Pix, Stride: r.frame.Stride, Rect: r.frame.Rect}
		r.frameB = &image.RGBA{Pix: r.frame.Pix, Stride: r.frame.Stride, Rect: r.frame.Rect}
		r.published = nil
		r.rebuildLayout(width, height)
		r.dirty = r.frame.Bounds()
	}
	now := time.Now()
	r.advanceAnimations(now)
	if err := r.advanceImageAnimations(r.layout, now); err != nil {
		return nil, err
	}
	if r.dirty.Empty() {
		return r.published, nil
	}
	dirty := r.dirty.Intersect(r.frame.Bounds())
	// 根节点会先用清屏色覆盖脏区，所以这里不需要额外清空。
	clear(r.world)
	r.paint(r.frame, r.layout, dirty, identity)
	if r.published == r.frameA {
		r.published = r.frameB
	} else {
		r.published = r.frameA
	}
	r.dirty = image.Rectangle{}
	return r.published, nil
}

func (r *Renderer) rebuildLayout(width, height int) {
	r.layout = layout.Compute(r.document.Root, width, height)
	r.boxes = make(map[string]*layout.Box)
	clear(r.maskCache)
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

func (r *Renderer) paint(frame *image.RGBA, box *layout.Box, limit image.Rectangle, parent affine) {
	node := box.Node
	if !node.Visible {
		return
	}
	transform := multiply(parent, nodeTransform(box, r.motions[node.ID]))
	r.world[node] = transform
	switch {
	case transform == identity:
		r.paintSelf(frame, box, limit)
	case isTranslation(transform):
		moved := translatedBox(box, transform[4], transform[5])
		r.paintSelf(frame, &moved, limit)
	case !box.Pixels.Empty():
		layer := r.layerFor(box)
		if clip := limit.Intersect(frame.Bounds()); !clip.Empty() {
			destination := frame
			if clip != frame.Bounds() {
				destination = frame.SubImage(clip).(*image.RGBA)
			}
			// 动画变换使用近似双线性，减少旋转和缩放时的 CPU 开销。
			xdraw.ApproxBiLinear.Transform(destination, toAff3(transform), layer, layer.Bounds(), draw.Over, nil)
		}
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
		fill := node.Color
		if node == r.hovered && node.Button != nil && !node.Button.Disabled && node.Button.HoverColor != nil {
			fill = *node.Button.HoverColor
		}
		r.fillRoundedRectClip(frame, box.Pixels, fill, node.Radius, limit)
	}
	if node.Image != nil && !box.Pixels.Intersect(frame.Bounds()).Intersect(limit).Empty() {
		var mask *image.Alpha
		if node.Radius > 0 {
			mask = r.roundedMask(box.Pixels.Size(), node.Radius)
		}
		source := node.Image.Image
		if player := r.imagePlayers[node]; player != nil {
			source = player.frame()
		}
		paintImage(frame, box.Pixels, limit, r.scaledImage(node, box.Pixels, source), node.Radius, mask)
	}
	if node.Text != nil {
		face := r.faces[faceKey{node.Text.Font, node.Text.FontSize}]
		paintText(frame, box.Pixels, node.Text, face, limit)
	}
}

func (r *Renderer) layerFor(box *layout.Box) *image.RGBA {
	if layer := r.layers[box.Node]; layer != nil && layer.Bounds() == box.Pixels {
		return layer
	}
	layer := image.NewRGBA(box.Pixels)
	r.paintSelf(layer, box, layer.Bounds())
	r.layers[box.Node] = layer
	return layer
}

func (r *Renderer) fillRoundedRectClip(dst *image.RGBA, rect image.Rectangle, fill color.NRGBA, radius float64, limit image.Rectangle) {
	clip := rect.Intersect(dst.Bounds()).Intersect(limit)
	if clip.Empty() || fill.A == 0 {
		return
	}
	if radius <= 0 {
		draw.Draw(dst, clip, image.NewUniform(fill), image.Point{}, draw.Over)
		return
	}
	mask := r.roundedMask(rect.Size(), radius)
	draw.DrawMask(dst, clip, image.NewUniform(fill), image.Point{}, mask, clip.Min.Sub(rect.Min), draw.Over)
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

// nodeVisible 同时检查节点和所有父节点的可见性。
func nodeVisible(root, target *ui.Node) bool {
	if !root.Visible {
		return false
	}
	if root == target {
		return true
	}
	for _, child := range root.Children {
		if nodeVisible(child, target) {
			return true
		}
	}
	return false
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
		delete(r.layers, node)
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
		delete(r.layers, node)
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
		delete(r.layers, node)
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
		if !visible {
			r.cancelUnavailablePointers()
		}
		r.resetImageAnimations(node)
		if box != nil {
			r.invalidate(layout.SubtreeBounds(box))
		}
	}
	return nil
}

// SetDisabled 修改按钮是否响应鼠标事件。
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
		if disabled {
			r.cancelUnavailablePointers()
		}
		delete(r.layers, node)
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
	r.frameA = nil
	r.frameB = nil
	r.published = nil
	r.layout = nil
	r.boxes = nil
	r.nodes = nil
	r.images = nil
	r.layers = nil
	r.maskCache = nil
	r.handlers = nil
	r.motions = nil
	r.imagePlayers = nil
	r.world = nil
	return errors.Join(errs...)
}
