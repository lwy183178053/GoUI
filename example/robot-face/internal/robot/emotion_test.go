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
		{Vector{2, 2}, "normal"},
		{Vector{1, 4}, "angry"},
		{Vector{4, 4}, "happy"},
		{Vector{1, 2}, "ponder"},
		{Vector{0, 0}, "sigh"}, // 空格子等距时采用登记顺序。
		{Vector{1, 1}, "mute"},
	} {
		got, err := nearestExpression(test.input)
		if err != nil || got.Name != test.name {
			t.Errorf("最近表情 %+v = %s, %v，期望 %s", test.input, got.Name, err, test.name)
		}
	}
}

func TestVectorRejectsOutOfRangeCoordinates(t *testing.T) {
	for _, input := range []Vector{{-1, 2}, {5, 2}, {2, -1}, {2, 5}} {
		if _, err := nearestExpression(input); err == nil {
			t.Errorf("非法坐标 %+v 没有报错", input)
		}
	}
}

// 五档只产生 25 个输入；每一格都必须有确定结果。
func TestEveryGridCellIsValid(t *testing.T) {
	for y := 0; y < GridSize; y++ {
		for x := 0; x < GridSize; x++ {
			got, err := nearestExpression(Vector{x, y})
			if err != nil || got.Name == "" {
				t.Fatalf("坐标 [%d,%d] 无法选择表情: %v", x, y, err)
			}
		}
	}
}
