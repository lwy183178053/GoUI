package main

import "errors"

// vector 的顺序始终是情绪值、唤醒度，两项都只接受 0～10 的整数。
type vector struct {
	Valence int `json:"valence"`
	Arousal int `json:"arousal"`
}

func (v vector) validate() error {
	if v.Valence < 0 || v.Valence > 10 || v.Arousal < 0 || v.Arousal > 10 {
		return errors.New("valence 和 arousal 都必须是 0～10 的整数")
	}
	return nil
}

// expression 给一张 GIF 登记一个坐标；坐标是本示例的可调配置。
type expression struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Valence int    `json:"valence"`
	Arousal int    `json:"arousal"`
	File    string `json:"file"`
}

var expressions = []expression{
	{Name: "normal", Label: "正常", Valence: 5, Arousal: 5, File: "smile_[0.5s].gif"},
	{Name: "happy", Label: "开心", Valence: 9, Arousal: 9, File: "laugh_[0.5s, 1.0s].gif"},
	{Name: "angry", Label: "生气", Valence: 1, Arousal: 9, File: "angry_[2.0s].gif"},
	{Name: "sad", Label: "难过", Valence: 1, Arousal: 2, File: "sad_[0.5s,1.5s].gif"},
	{Name: "surprised", Label: "惊讶", Valence: 5, Arousal: 10, File: "shocked_[0.5s, 1.0s].gif"},
	{Name: "question", Label: "疑惑", Valence: 5, Arousal: 6, File: "question_[0.5s].gif"},
	{Name: "asleep", Label: "睡觉", Valence: 5, Arousal: 0, File: "asleep_[21.5s].gif"},
	{Name: "badminton", Label: "运动", Valence: 8, Arousal: 8, File: "badminton_[1.2s].gif"},
	{Name: "confident", Label: "自信", Valence: 9, Arousal: 6, File: "confident_[0.8s].gif"},
	{Name: "cry", Label: "哭泣", Valence: 0, Arousal: 5, File: "cry_[1.0s, 2.0s].gif"},
	{Name: "investigate", Label: "观察", Valence: 6, Arousal: 6, File: "investigate_[0.5s, 2.5s].gif"},
	{Name: "leisure", Label: "悠闲", Valence: 7, Arousal: 2, File: "leisure_[0.5s, 2.0s].gif"},
	{Name: "mock", Label: "调皮", Valence: 4, Arousal: 8, File: "mock_[0.5s].gif"},
	{Name: "music", Label: "音乐", Valence: 8, Arousal: 6, File: "music_[2.5s].gif"},
	{Name: "mute", Label: "静音", Valence: 5, Arousal: 2, File: "mute_[0.5s].gif"},
	{Name: "panic", Label: "惊恐", Valence: 2, Arousal: 10, File: "panic_[0.5s, 1.5s].gif"},
	{Name: "ponder", Label: "思考", Valence: 4, Arousal: 5, File: "ponder_[0.5s].gif"},
	{Name: "shy", Label: "害羞", Valence: 7, Arousal: 4, File: "shy_[2.0s, 4.0s].gif"},
	{Name: "sigh", Label: "叹气", Valence: 3, Arousal: 3, File: "sigh_[2.0s, 4.0s].gif"},
	{Name: "smile-static", Label: "微笑", Valence: 7, Arousal: 1, File: "smile_static_[0.5s].gif"},
	{Name: "snigger", Label: "坏笑", Valence: 9, Arousal: 7, File: "snigger_[1.0s].gif"},
	{Name: "yawn", Label: "困倦", Valence: 4, Arousal: 1, File: "yawn_[2.0s].gif"},
	{Name: "yummy", Label: "美味", Valence: 9, Arousal: 4, File: "yummy_[2.0 s].gif"},
}

func nearestExpression(input vector) (expression, error) {
	if err := input.validate(); err != nil {
		return expression{}, err
	}
	nearest := expressions[0]
	bestDistance := 201 // 两轴范围都是 0～10，距离平方最大为 200。
	for _, item := range expressions {
		dx := input.Valence - item.Valence
		dy := input.Arousal - item.Arousal
		distance := dx*dx + dy*dy
		// 等距时保留列表里较早的一项，让结果始终一致。
		if distance < bestDistance {
			nearest, bestDistance = item, distance
		}
	}
	return nearest, nil
}
