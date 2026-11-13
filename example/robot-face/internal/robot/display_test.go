package robot

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func text(value string) *string { return &value }

func TestDisplayReplacementAppendAndStatusDoNotChangeEmotion(t *testing.T) {
	controller, _ := testRobot(t)
	before := controller.Emotion()
	for _, test := range []struct {
		update DisplayUpdate
		want   DisplayState
	}{
		{DisplayUpdate{Text: text("你好"), Status: text("speaking")}, DisplayState{Text: "你好", Status: "speaking"}},
		{DisplayUpdate{Append: text("，我正在回答。")}, DisplayState{Text: "你好，我正在回答。", Status: "speaking"}},
		{DisplayUpdate{Status: text("listening")}, DisplayState{Text: "你好，我正在回答。", Status: "listening"}},
		{DisplayUpdate{Text: text(""), Status: text("idle")}, DisplayState{Status: "idle"}},
	} {
		got, err := controller.SetDisplay(context.Background(), test.update)
		if err != nil || got != test.want || controller.Display() != test.want {
			t.Fatalf("显示更新失败：%+v %v", got, err)
		}
		if controller.Emotion() != before {
			t.Fatal("文字状态不应强制修改表情")
		}
	}
}

func TestDisplayRejectsInvalidUpdatesAndKeepsState(t *testing.T) {
	controller, _ := testRobot(t)
	before := controller.Display()
	for _, update := range []DisplayUpdate{
		{}, {Text: text("a"), Append: text("b")}, {Status: text("unknown")},
		{Text: text(strings.Repeat("字", MaxTextLength+1))}, {Text: text(string([]byte{0xff}))},
	} {
		if _, err := controller.SetDisplay(context.Background(), update); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("非法输入应返回业务错误：%v", err)
		}
		if controller.Display() != before {
			t.Fatal("失败请求修改了文字状态")
		}
	}
	if _, err := controller.SetDisplay(context.Background(), DisplayUpdate{Text: text(strings.Repeat("字", MaxTextLength))}); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.SetDisplay(context.Background(), DisplayUpdate{Append: text("多")}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("增量也应校验累计长度")
	}
}

func TestDisplayConcurrentAppendDoesNotLoseChunks(t *testing.T) {
	controller, _ := testRobot(t)
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() {
			if _, err := controller.SetDisplay(context.Background(), DisplayUpdate{Append: text("好")}); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	if controller.Display().Text != strings.Repeat("好", 20) {
		t.Fatal("并发追加丢失文字")
	}
}

func TestTextSurvivesExpressionSourceSwitch(t *testing.T) {
	controller, _ := testRobot(t)
	want := DisplayState{Text: "这段回答在切换表情后仍保留", Status: "speaking"}
	if _, err := controller.SetDisplay(context.Background(), DisplayUpdate{Text: text(want.Text), Status: text(want.Status)}); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.SetEmotion(context.Background(), Vector{9, 9}); err != nil {
		t.Fatal(err)
	}
	if err := controller.showTarget(nil); err != nil {
		t.Fatal(err)
	}
	if controller.Display() != want || controller.Emotion().Expression.Name != "happy" {
		t.Fatal("表情切换丢失文字或切换失败")
	}
}
