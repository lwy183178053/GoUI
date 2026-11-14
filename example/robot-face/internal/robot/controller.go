package robot

import (
	"context"
	"errors"
	"sync"
	"time"

	goui "github.com/lwy183178053/GoUI"
)

// ExpressionRequest 指定动作名称、是否循环，以及是否立即打断当前动作。
type ExpressionRequest struct {
	Name      string `json:"name"`
	Loop      bool   `json:"loop"`
	Interrupt bool   `json:"interrupt"`
}

// EmotionState 保存实际表情和一个待切换请求，不积压动作队列。
// 正常表情始终按随机间隔眨眼，Loop 为 false。
type EmotionState struct {
	Expression Expression         `json:"expression"`
	Loop       bool               `json:"loop"`
	Pending    *ExpressionRequest `json:"pending,omitempty"`
}

// Controller 操作项目 XML 中的表情、底部文字和麦克风按钮，框架负责解码与绘制。
// 组件通过 dispatch 在窗口主循环中修改；mu 保护业务状态、眨眼计划和文字进度。
type Controller struct {
	mu          sync.RWMutex
	current     EmotionState
	display     DisplayState
	typing      typewriter
	generation  uint64
	blinkAt     time.Time
	effectsWake chan struct{}
	face        *goui.Image
	caption     *goui.Text
	microphone  *goui.Button
	micClicks   chan struct{}
	dispatch    func(context.Context, func(*goui.Page) error) error
}

// New 绑定项目组件并播放一次正常眨眼，不预先加载其它候选 GIF。
// 随机眨眼与逐字显示由 RunEffects 管理，初始化不启动后台任务。
func New(page *goui.Page, dispatch func(context.Context, func(*goui.Page) error) error) (*Controller, error) {
	if dispatch == nil {
		return nil, errors.New("机器人控制器需要页面调度函数")
	}
	face, err := page.GetImage("face")
	if err != nil {
		return nil, err
	}
	caption, err := page.GetText("caption")
	if err != nil {
		return nil, err
	}
	microphone, err := page.GetButton("microphone")
	if err != nil {
		return nil, err
	}
	controller := &Controller{
		face: face, caption: caption, microphone: microphone, dispatch: dispatch,
		micClicks:   make(chan struct{}, 1),
		effectsWake: make(chan struct{}, 1),
		current:     EmotionState{Expression: expressions[0]},
	}
	if err := microphone.OnClick(controller.onMicrophoneClick); err != nil {
		return nil, err
	}
	if err := controller.playExpression(expressions[0], false); err != nil {
		return nil, err
	}
	return controller, nil
}

// Emotion 返回当前表情状态的快照。
func (c *Controller) Emotion() EmotionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.emotionSnapshot()
}

// emotionSnapshot 在持有 mu 时调用，避免调用方修改内部的待切换请求。
func (c *Controller) emotionSnapshot() EmotionState {
	state := c.current
	if state.Pending != nil {
		pending := *state.Pending
		state.Pending = &pending
	}
	return state
}
