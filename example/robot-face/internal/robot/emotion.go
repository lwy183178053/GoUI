// Package robot 只实现机器人项目的表情与显示业务，不属于 GoUI 框架。
package robot

import "fmt"

// Expression 将接口名称、中文显示名和 GIF 文件对应起来。
// Name 是稳定的调用名称，修改素材文件名不会改变外部接口。
type Expression struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	File  string `json:"file"`
}

var expressions = []Expression{
	{Name: "normal", Label: "正常", File: "smile_[0.5s].gif"},
	{Name: "happy", Label: "开心", File: "laugh_[0.5s, 1.0s].gif"},
	{Name: "angry", Label: "生气", File: "angry_[2.0s].gif"},
	{Name: "sad", Label: "难过", File: "sad_[0.5s,1.5s].gif"},
	{Name: "surprised", Label: "惊讶", File: "shocked_[0.5s, 1.0s].gif"},
	{Name: "question", Label: "疑惑", File: "question_[0.5s].gif"},
	{Name: "asleep", Label: "睡觉", File: "asleep_[21.5s].gif"},
	{Name: "badminton", Label: "运动", File: "badminton_[1.2s].gif"},
	{Name: "confident", Label: "自信", File: "confident_[0.8s].gif"},
	{Name: "cry", Label: "哭泣", File: "cry_[1.0s, 2.0s].gif"},
	{Name: "investigate", Label: "观察", File: "investigate_[0.5s, 2.5s].gif"},
	{Name: "leisure", Label: "悠闲", File: "leisure_[0.5s, 2.0s].gif"},
	{Name: "mock", Label: "调皮", File: "mock_[0.5s].gif"},
	{Name: "music", Label: "音乐", File: "music_[2.5s].gif"},
	{Name: "mute", Label: "静音", File: "mute_[0.5s].gif"},
	{Name: "panic", Label: "惊恐", File: "panic_[0.5s, 1.5s].gif"},
	{Name: "ponder", Label: "思考", File: "ponder_[0s,1.52s].gif"},
	{Name: "shy", Label: "害羞", File: "shy_[2.0s, 4.0s].gif"},
	{Name: "sigh", Label: "叹气", File: "sigh_[2.0s, 4.0s].gif"},
	{Name: "smile-static", Label: "微笑", File: "smile_static_[0.5s].gif"},
	{Name: "snigger", Label: "坏笑", File: "snigger_[1.0s].gif"},
	{Name: "yawn", Label: "困倦", File: "yawn_[2.0s].gif"},
	{Name: "yummy", Label: "美味", File: "yummy_[2.0 s].gif"},
}

func findExpression(name string) (Expression, error) {
	for _, expression := range expressions {
		if expression.Name == name {
			return expression, nil
		}
	}
	return Expression{}, fmt.Errorf("%w: 未知表情 %q，请从 /api/expressions 获取可用名称", ErrInvalidInput, name)
}

// Expressions 返回登记表的副本，调用方不能修改项目内的配置。
func Expressions() []Expression {
	return append([]Expression(nil), expressions...)
}
