// Package layout 计算布局，不依赖窗口或绘制后端。
package layout

import (
	"image"
	"math"

	"github.com/lwy183178053/GoUI/internal/ui"
)

type Rect struct{ X, Y, Width, Height float64 }

// Box 同时保存小数布局和绘制使用的整数像素矩形。
type Box struct {
	Node     *ui.Node
	Geometry Rect
	Pixels   image.Rectangle
	Children []*Box
}

func Compute(root *ui.Node, width, height int) *Box {
	return build(root, Rect{Width: float64(width), Height: float64(height)})
}

func build(node *ui.Node, geometry Rect) *Box {
	box := &Box{
		Node:     node,
		Geometry: geometry,
		Pixels: image.Rect(
			int(math.Round(geometry.X)), int(math.Round(geometry.Y)),
			int(math.Round(geometry.X+geometry.Width)), int(math.Round(geometry.Y+geometry.Height)),
		),
	}
	for _, child := range node.Children {
		x, width := axis(geometry.X, geometry.Width, child.Anchor.X, child.Pivot.X, child.Position.X, child.Size.X, child.Margin.Left, child.Margin.Right)
		y, height := axis(geometry.Y, geometry.Height, child.Anchor.Y, child.Pivot.Y, child.Position.Y, child.Size.Y, child.Margin.Top, child.Margin.Bottom)
		box.Children = append(box.Children, build(child, Rect{x, y, width, height}))
	}
	return box
}

func axis(origin, length, anchor, pivot float64, position, size, before, after int) (float64, float64) {
	if size == -1 {
		return origin + float64(before), math.Max(0, length-float64(before)-float64(after))
	}
	return origin + length*anchor + float64(position) - float64(size)*pivot, float64(size)
}

// HitButton 从上到下查找最上层按钮，普通节点不拦截命中。
func HitButton(box *Box, x, y int) *ui.Node {
	if box == nil || !box.Node.Visible {
		return nil
	}
	for i := len(box.Children) - 1; i >= 0; i-- {
		if button := HitButton(box.Children[i], x, y); button != nil {
			return button
		}
	}
	if box.Node.Button != nil && image.Pt(x, y).In(box.Pixels) {
		return box.Node
	}
	return nil
}

// SubtreeBounds 返回节点及其子树在画布中的总范围。
func SubtreeBounds(box *Box) image.Rectangle {
	if box == nil {
		return image.Rectangle{}
	}
	result := box.Pixels
	for _, child := range box.Children {
		result = result.Union(SubtreeBounds(child))
	}
	return result
}
