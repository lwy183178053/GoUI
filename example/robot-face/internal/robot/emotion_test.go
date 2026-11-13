package robot

import "testing"

func TestExpressionCoordinatesAreUniqueAndReachable(t *testing.T) {
	coordinates := make(map[Vector]string)
	names := make(map[string]bool)
	for _, item := range expressions {
		point := Vector{item.Valence, item.Arousal}
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
		input Vector
		name  string
	}{
		{Vector{5, 5}, "normal"},
		{Vector{2, 8}, "angry"},
		{Vector{0, 0}, "sad"},
		{Vector{10, 10}, "happy"},
		{Vector{8, 9}, "happy"},  // 开心和运动等距，采用登记顺序。
		{Vector{5, 1}, "asleep"}, // 睡觉、静音和困倦等距，采用登记顺序。
	} {
		got, err := nearestExpression(test.input)
		if err != nil || got.Name != test.name {
			t.Errorf("最近表情 %+v = %s, %v，期望 %s", test.input, got.Name, err, test.name)
		}
	}
}

func TestVectorRejectsOutOfRangeCoordinates(t *testing.T) {
	for _, input := range []Vector{{-1, 5}, {11, 5}, {5, -1}, {5, 11}} {
		if _, err := nearestExpression(input); err == nil {
			t.Errorf("非法坐标 %+v 没有报错", input)
		}
	}
}
