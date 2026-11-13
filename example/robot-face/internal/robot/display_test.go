package robot

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func advanceCaption(t *testing.T, controller *Controller, elapsed time.Duration) {
	t.Helper()
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if err := controller.advanceText(controller.typing.started.Add(elapsed)); err != nil {
		t.Fatal(err)
	}
}

func TestDisplayTypesUnicodeAtTheConfiguredInterval(t *testing.T) {
	controller, _ := testRobot(t)
	before := controller.Emotion()
	got, err := controller.SetDisplay(context.Background(), "你🙂好！")
	want := DisplayState{Text: "你🙂好！", VisibleText: "你", Typing: true}
	if err != nil || got != want || controller.Display() != want {
		t.Fatalf("首字应立即显示并保留全文：%+v %v", got, err)
	}
	for _, step := range []struct {
		elapsed time.Duration
		visible string
		typing  bool
	}{
		{39 * time.Millisecond, "你", true},
		{40 * time.Millisecond, "你🙂", true},
		{80 * time.Millisecond, "你🙂好", true},
		{120 * time.Millisecond, "你🙂好！", false},
		{time.Hour, "你🙂好！", false},
	} {
		advanceCaption(t, controller, step.elapsed)
		state := controller.Display()
		if state.Text != want.Text || state.VisibleText != step.visible || state.Typing != step.typing {
			t.Fatalf("%s 的显示进度不正确：%+v", step.elapsed, state)
		}
	}
	if !controller.nextTextTime().IsZero() || controller.Emotion() != before {
		t.Fatal("打字结束后不再调度文字，也不改变表情")
	}
}

func TestDisplayReplacementAndClearCancelPreviousText(t *testing.T) {
	controller, _ := testRobot(t)
	for _, value := range []string{"原来的长句子", "全新的回答"} {
		if _, err := controller.SetDisplay(context.Background(), value); err != nil {
			t.Fatal(err)
		}
		if state := controller.Display(); state.Text != value || state.VisibleText != string([]rune(value)[:1]) {
			t.Fatalf("新句子必须替换并从首字开始：%+v", state)
		}
	}
	// 窗口短暂忙碌后直接追上进度，不能恢复被替换的旧内容。
	advanceCaption(t, controller, time.Second)
	if state := controller.Display(); state.VisibleText != "全新的回答" || state.Typing {
		t.Fatalf("完成后不应出现旧句子：%+v", state)
	}
	if _, err := controller.SetDisplay(context.Background(), "再次开始"); err != nil {
		t.Fatal(err)
	}
	if state, err := controller.SetDisplay(context.Background(), ""); err != nil || state != (DisplayState{}) {
		t.Fatalf("清空应立即生效：%+v %v", state, err)
	}
	advanceCaption(t, controller, time.Hour)
	if controller.Display() != (DisplayState{}) || !controller.nextTextTime().IsZero() {
		t.Fatal("清空后旧打字任务不能重新显示文字")
	}
}

func TestDisplaySingleCharacterNeedsNoTimer(t *testing.T) {
	controller, _ := testRobot(t)
	state, err := controller.SetDisplay(context.Background(), "好")
	if err != nil || state.Text != "好" || state.VisibleText != "好" || state.Typing || !controller.nextTextTime().IsZero() {
		t.Fatalf("单字应立即完成：%+v %v", state, err)
	}
}

func TestDisplayRejectsInvalidTextAndKeepsProgress(t *testing.T) {
	controller, _ := testRobot(t)
	if _, err := controller.SetDisplay(context.Background(), "正在显示的句子"); err != nil {
		t.Fatal(err)
	}
	advanceCaption(t, controller, 40*time.Millisecond)
	before := controller.Display()
	deadline := controller.nextTextTime()
	for _, value := range []string{strings.Repeat("字", MaxTextLength+1), string([]byte{0xff})} {
		if _, err := controller.SetDisplay(context.Background(), value); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("非法输入应返回业务错误：%v", err)
		}
		if controller.Display() != before || !controller.nextTextTime().Equal(deadline) {
			t.Fatal("失败请求不能重置正在显示的文字")
		}
	}
	if _, err := controller.SetDisplay(context.Background(), strings.Repeat("字", MaxTextLength)); err != nil {
		t.Fatal(err)
	}
}

func TestTextSurvivesExpressionSourceSwitch(t *testing.T) {
	controller, _ := testRobot(t)
	if _, err := controller.SetDisplay(context.Background(), "这段文字在切换表情后继续显示"); err != nil {
		t.Fatal(err)
	}
	before := controller.Display()
	deadline := controller.nextTextTime()
	if _, err := controller.SetExpression(context.Background(), ExpressionRequest{Name: "happy", Interrupt: true}); err != nil {
		t.Fatal(err)
	}
	if controller.Display() != before || !controller.nextTextTime().Equal(deadline) || controller.Emotion().Expression.Name != "happy" {
		t.Fatal("切换表情不应重置文字或打字进度")
	}
	advanceCaption(t, controller, time.Second)
	if state := controller.Display(); state.Typing || state.VisibleText != before.Text {
		t.Fatalf("切换后文字应继续打完：%+v", state)
	}
}
