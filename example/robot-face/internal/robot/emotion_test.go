package robot

import (
	"errors"
	"testing"
)

func TestExpressionNamesAreUniqueAndCanBeFound(t *testing.T) {
	names := make(map[string]bool)
	for _, item := range expressions {
		if item.Name == "" || item.Label == "" || item.File == "" || names[item.Name] {
			t.Fatalf("表情配置缺失或名称重复：%+v", item)
		}
		names[item.Name] = true
		got, err := findExpression(item.Name)
		if err != nil || got != item {
			t.Fatalf("按名查找 %s 失败：%+v, %v", item.Name, got, err)
		}
	}
}

func TestUnknownExpressionIsAnInputError(t *testing.T) {
	for _, name := range []string{"", "unknown", "MUSIC", " music ", "音乐"} {
		if _, err := findExpression(name); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("未知名称 %q 应报告输入错误：%v", name, err)
		}
	}
}

func TestExpressionListIsACopy(t *testing.T) {
	items := Expressions()
	items[0].Name = "changed"
	if expressions[0].Name != "normal" {
		t.Fatal("调用方不能修改内部表情登记表")
	}
}
