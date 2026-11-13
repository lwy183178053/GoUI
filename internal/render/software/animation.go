package software

import (
	"errors"
	"image"
	"math"
	"time"

	"github.com/lwy183178053/GoUI/internal/layout"
	"github.com/lwy183178053/GoUI/internal/ui"

	"golang.org/x/image/math/f64"
)

type tween struct {
	from     [2]float64
	to       [2]float64
	started  time.Time
	duration time.Duration
}

func (t *tween) sample(now time.Time) ([2]float64, bool) {
	progress := float64(now.Sub(t.started)) / float64(t.duration)
	if progress >= 1 {
		return t.to, false
	}
	if progress < 0 {
		progress = 0
	}
	return [2]float64{
		t.from[0] + (t.to[0]-t.from[0])*progress,
		t.from[1] + (t.to[1]-t.from[1])*progress,
	}, true
}

type animation struct {
	position [2]float64
	rotation float64
	scale    [2]float64
	move     *tween
	rotate   *tween
	zoom     *tween
}

// affine 使用 a,b,c,d,e,f 表示 x'=a*x+c*y+e、y'=b*x+d*y+f。
type affine [6]float64

var identity = affine{1, 0, 0, 1, 0, 0}

func (r *Renderer) motion(id string, duration time.Duration) (*ui.Node, *animation, time.Time, error) {
	if duration < 0 {
		return nil, nil, time.Time{}, errors.New("animation duration cannot be negative")
	}
	node, _, err := r.node(id)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	if _, exists := r.motions[id]; !exists {
		r.motions[id] = &animation{
			position: [2]float64{float64(node.Position.X), float64(node.Position.Y)},
			scale:    [2]float64{1, 1},
		}
	}
	now := time.Now()
	r.advanceAnimations(now)
	return node, r.motions[id], now, nil
}

// MoveTo 在指定时长内改变 position；size=-1 的轴仍按 margin 布局，不参与位移。
func (r *Renderer) MoveTo(id string, x, y int, duration time.Duration) error {
	node, motion, now, err := r.motion(id, duration)
	if err != nil {
		return err
	}
	target := [2]float64{float64(x), float64(y)}
	if duration == 0 {
		motion.position = target
		motion.move = nil
		node.Position = image.Pt(x, y)
		r.animationChanged(true)
		return nil
	}
	motion.move = &tween{from: motion.position, to: target, started: now, duration: duration}
	return nil
}

// RotateTo 在指定时长内顺时针旋转到目标角度。
func (r *Renderer) RotateTo(id string, degrees float64, duration time.Duration) error {
	if math.IsNaN(degrees) || math.IsInf(degrees, 0) {
		return errors.New("rotation must be a finite number")
	}
	_, motion, now, err := r.motion(id, duration)
	if err != nil {
		return err
	}
	if duration == 0 {
		motion.rotation = degrees
		motion.rotate = nil
		r.animationChanged(false)
		return nil
	}
	motion.rotate = &tween{from: [2]float64{motion.rotation}, to: [2]float64{degrees}, started: now, duration: duration}
	return nil
}

// ScaleTo 在指定时长内围绕 pivot 缩放，1 表示原始大小。
func (r *Renderer) ScaleTo(id string, x, y float64, duration time.Duration) error {
	if !finitePositive(x) || !finitePositive(y) {
		return errors.New("scale values must be finite numbers greater than zero")
	}
	_, motion, now, err := r.motion(id, duration)
	if err != nil {
		return err
	}
	target := [2]float64{x, y}
	if duration == 0 {
		motion.scale = target
		motion.zoom = nil
		r.animationChanged(false)
		return nil
	}
	motion.zoom = &tween{from: motion.scale, to: target, started: now, duration: duration}
	return nil
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (r *Renderer) advanceAnimations(now time.Time) {
	changed, moved := false, false
	for id, motion := range r.motions {
		if motion.move != nil {
			value, running := motion.move.sample(now)
			if motion.position != value {
				motion.position = value
				changed = true
			}
			if !running {
				motion.move = nil
				node := r.nodes[id]
				position := image.Pt(int(value[0]), int(value[1]))
				if node.Position != position {
					node.Position = position
					moved = true
				}
			}
		}
		if motion.rotate != nil {
			value, running := motion.rotate.sample(now)
			if motion.rotation != value[0] {
				motion.rotation = value[0]
				changed = true
			}
			if !running {
				motion.rotate = nil
			}
		}
		if motion.zoom != nil {
			value, running := motion.zoom.sample(now)
			if motion.scale != value {
				motion.scale = value
				changed = true
			}
			if !running {
				motion.zoom = nil
			}
		}
	}
	if moved && r.frame != nil {
		r.rebuildLayout(r.frame.Rect.Dx(), r.frame.Rect.Dy())
	}
	if changed && r.frame != nil {
		r.invalidate(r.frame.Bounds())
	}
}

func (r *Renderer) animationChanged(moved bool) {
	if r.frame == nil {
		return
	}
	if moved {
		r.rebuildLayout(r.frame.Rect.Dx(), r.frame.Rect.Dy())
	}
	r.invalidate(r.frame.Bounds())
}

func nodeTransform(box *layout.Box, motion *animation) affine {
	if motion == nil {
		return identity
	}
	angle := motion.rotation * math.Pi / 180
	cosine, sine := math.Cos(angle), math.Sin(angle)
	a := cosine * motion.scale[0]
	b := sine * motion.scale[0]
	c := -sine * motion.scale[1]
	d := cosine * motion.scale[1]
	x := box.Geometry.X + box.Geometry.Width*box.Node.Pivot.X
	y := box.Geometry.Y + box.Geometry.Height*box.Node.Pivot.Y
	transform := affine{a, b, c, d, x - a*x - c*y, y - b*x - d*y}
	// 填充轴始终忽略 position，避免动画中途移走、结束后又被布局拉回。
	if box.Node.Size.X != -1 {
		transform[4] += motion.position[0] - float64(box.Node.Position.X)
	}
	if box.Node.Size.Y != -1 {
		transform[5] += motion.position[1] - float64(box.Node.Position.Y)
	}
	return transform
}

func isTranslation(transform affine) bool {
	return transform[0] == 1 && transform[1] == 0 && transform[2] == 0 && transform[3] == 1
}

func translatedBox(box *layout.Box, x, y float64) layout.Box {
	moved := *box
	moved.Geometry.X += x
	moved.Geometry.Y += y
	moved.Pixels = image.Rect(
		int(math.Round(moved.Geometry.X)),
		int(math.Round(moved.Geometry.Y)),
		int(math.Round(moved.Geometry.X+moved.Geometry.Width)),
		int(math.Round(moved.Geometry.Y+moved.Geometry.Height)),
	)
	return moved
}

func multiply(outer, inner affine) affine {
	return affine{
		outer[0]*inner[0] + outer[2]*inner[1],
		outer[1]*inner[0] + outer[3]*inner[1],
		outer[0]*inner[2] + outer[2]*inner[3],
		outer[1]*inner[2] + outer[3]*inner[3],
		outer[0]*inner[4] + outer[2]*inner[5] + outer[4],
		outer[1]*inner[4] + outer[3]*inner[5] + outer[5],
	}
}

func toAff3(matrix affine) f64.Aff3 {
	return f64.Aff3{matrix[0], matrix[2], matrix[4], matrix[1], matrix[3], matrix[5]}
}

func inversePoint(matrix affine, x, y float64) (float64, float64) {
	determinant := matrix[0]*matrix[3] - matrix[1]*matrix[2]
	if determinant == 0 {
		return x, y
	}
	x -= matrix[4]
	y -= matrix[5]
	return (matrix[3]*x - matrix[2]*y) / determinant,
		(matrix[0]*y - matrix[1]*x) / determinant
}
