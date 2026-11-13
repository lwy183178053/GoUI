package robot

import (
	"context"
	"errors"
	"sync"

	goui "github.com/lwy183178053/GoUI"
)

// EmotionState 区分当前表情和最新目标，回正期间两者可能不同。
type EmotionState struct {
	Vector        Vector     `json:"vector"`
	Expression    Expression `json:"expression"`
	Target        Expression `json:"target"`
	Transitioning bool       `json:"transitioning"`
}

// Controller 操作项目 XML 中的一个 Image 和底部文字，框架负责解码与绘制。
// 组件通过 dispatch 在窗口主循环中修改；mu 只保护 HTTP 读取的业务状态。
type Controller struct {
	mu       sync.RWMutex
	current  EmotionState
	display  DisplayState
	face     *goui.Image
	caption  *goui.Block
	answer   *goui.Text
	status   *goui.Text
	dispatch func(context.Context, func(*goui.Page) error) error
}

// New 绑定项目自己的组件，不预先加载其它候选 GIF。
func New(page *goui.Page, dispatch func(context.Context, func(*goui.Page) error) error) (*Controller, error) {
	if dispatch == nil {
		return nil, errors.New("机器人控制器需要页面调度函数")
	}
	face, err := page.GetImage("face")
	if err != nil {
		return nil, err
	}
	caption, err := page.GetBlock("caption")
	if err != nil {
		return nil, err
	}
	answer, err := page.GetText("answer")
	if err != nil {
		return nil, err
	}
	status, err := page.GetText("status")
	if err != nil {
		return nil, err
	}
	initial := Vector{Valence: 5, Arousal: 5}
	normal, err := nearestExpression(initial)
	if err != nil {
		return nil, err
	}
	return &Controller{
		face: face, caption: caption, answer: answer, status: status, dispatch: dispatch,
		current: EmotionState{Vector: initial, Expression: normal, Target: normal},
		display: DisplayState{Status: "idle"},
	}, nil
}

// Emotion 返回当前表情状态的快照。
func (c *Controller) Emotion() EmotionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// SetEmotion 提交向量；回正期间只替换最新目标，不排队播放旧目标。
func (c *Controller) SetEmotion(ctx context.Context, input Vector) (EmotionState, error) {
	next, err := nearestExpression(input)
	if err != nil {
		return EmotionState{}, err
	}
	var result EmotionState
	err = c.dispatch(ctx, func(*goui.Page) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.current.Transitioning && next.Name != c.current.Expression.Name {
			if err := c.face.Finish(c.showTarget); err != nil {
				return err
			}
			c.current.Transitioning = true
		}
		c.current.Vector = input
		c.current.Target = next
		result = c.current
		return nil
	})
	if err != nil {
		return EmotionState{}, err
	}
	return result, nil
}

// showTarget 已在主循环中，直接换源；Image 子节点及底部回答不会被重建。
func (c *Controller) showTarget(*goui.Page) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.current.Transitioning {
		return nil
	}
	if err := c.face.SetSource("gif/" + c.current.Target.File); err != nil {
		return err
	}
	c.current.Expression = c.current.Target
	c.current.Transitioning = false
	return nil
}
