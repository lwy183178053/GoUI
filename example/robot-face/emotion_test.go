package main

import "testing"

func TestExpressionCoordinatesAreUniqueAndReachable(t *testing.T) {
	coordinates := make(map[vector]string)
	names := make(map[string]bool)
	for _, item := range expressions {
		point := vector{item.Valence, item.Arousal}
		if previous, exists := coordinates[point]; exists {
			t.Errorf("%s 和 %s 使用了相同坐标 %+v", previous, item.Name, point)
		}
		if names[item.Name] {
			t.Errorf("表情名重复：%s", item.Name)
		}
		coordinates[point], names[item.Name] = item.Name, true
		got, err := nearestExpression(point)
		if err != nil || got.Name != item.Name {
			t.Errorf("坐标 %+v = %s, %v，期望 %s", point, got.Name, err, item.Name)
		}
	}
}

func TestNearestExpressionAndStableTie(t *testing.T) {
	for _, test := range []struct {
		input vector
		name  string
	}{
		{vector{5, 5}, "normal"},
		{vector{2, 8}, "angry"},
		{vector{0, 0}, "sad"},
		{vector{10, 10}, "happy"},
		{vector{8, 9}, "happy"},  // 开心和运动等距，采用登记顺序。
		{vector{5, 1}, "asleep"}, // 睡觉、静音和困倦等距，采用登记顺序。
	} {
		got, err := nearestExpression(test.input)
		if err != nil || got.Name != test.name {
			t.Errorf("最近表情 %+v = %s, %v，期望 %s", test.input, got.Name, err, test.name)
		}
	}
}

func TestVectorRejectsOutOfRangeCoordinates(t *testing.T) {
	for _, input := range []vector{{-1, 5}, {11, 5}, {5, -1}, {5, 11}} {
		if _, err := nearestExpression(input); err == nil {
			t.Errorf("非法坐标 %+v 没有报错", input)
		}
	}
}
