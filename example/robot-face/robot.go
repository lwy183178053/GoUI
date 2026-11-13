package main

import (
	"context"
	"sync"

	goui "github.com/lwy183178053/GoUI"
)

// emotionState 区分当前正在显示的表情和最新目标；回正时两者可能不同。
type emotionState struct {
	Vector        vector     `json:"vector"`
	Expression    expression `json:"expression"`
	Target        expression `json:"target"`
	Transitioning bool       `json:"transitioning"`
}

// robot 管理表情选择与切换，GIF 的解码、分段和计时仍由框架负责。
// mu 保护 HTTP 读取的状态，组件操作通过 dispatch 留在窗口主循环中。
type robot struct {
	mu       sync.RWMutex
	current  emotionState
	images   map[string]*goui.Image
	dispatch func(context.Context, func(*goui.Page) error) error
}

func newRobot(page *goui.Page, dispatch func(context.Context, func(*goui.Page) error) error) (*robot, error) {
	initial := vector{Valence: 5, Arousal: 5}
	normal, err := nearestExpression(initial)
	if err != nil {
		return nil, err
	}
	robot := &robot{images: make(map[string]*goui.Image), dispatch: dispatch}
	for _, item := range expressions {
		image, err := page.GetImage("expression-" + item.Name)
		if err != nil {
			return nil, err
		}
		robot.images[item.Name] = image
		if err := image.SetVisible(item.Name == normal.Name); err != nil {
			return nil, err
		}
	}
	robot.current = emotionState{Vector: initial, Expression: normal, Target: normal}
	return robot, nil
}

func (robot *robot) state() emotionState {
	robot.mu.RLock()
	defer robot.mu.RUnlock()
	return robot.current
}

func (robot *robot) set(ctx context.Context, input vector) (emotionState, error) {
	next, err := nearestExpression(input)
	if err != nil {
		return emotionState{}, err
	}
	var result emotionState
	err = robot.dispatch(ctx, func(*goui.Page) error {
		robot.mu.Lock()
		defer robot.mu.Unlock()
		if !robot.current.Transitioning && next.Name != robot.current.Expression.Name {
			if err := robot.images[robot.current.Expression.Name].Finish(robot.showTarget); err != nil {
				return err
			}
			robot.current.Transitioning = true
		}
		// 回正期间只更新最新目标，不重复请求退出，也不排队播放旧目标。
		robot.current.Vector = input
		robot.current.Target = next
		result = robot.current
		return nil
	})
	if err != nil {
		return emotionState{}, err
	}
	return result, nil
}

// showTarget 是 GIF 完成回正后的回调，已经在主循环中，无需再次 Dispatch。
func (robot *robot) showTarget(*goui.Page) error {
	robot.mu.Lock()
	defer robot.mu.Unlock()
	if !robot.current.Transitioning {
		return nil
	}
	if err := robot.images[robot.current.Expression.Name].SetVisible(false); err != nil {
		return err
	}
	if err := robot.images[robot.current.Target.Name].SetVisible(true); err != nil {
		return err
	}
	robot.current.Expression = robot.current.Target
	robot.current.Transitioning = false
	return nil
}
