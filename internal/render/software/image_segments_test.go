package software

import (
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/lwy183178053/GoUI/internal/ui"
)

func segmentedPlayback(t *testing.T, start, end time.Duration) *imagePlayback {
	t.Helper()
	animation := testGIFAnimation(t, -1, 200*time.Millisecond, 300*time.Millisecond, 100*time.Millisecond, 400*time.Millisecond)
	animation.Segments = &ui.GIFSegments{Start: start, End: end}
	return newImagePlayback(animation)
}

func TestSegmentedPlaybackHoldsOrLoopsWithoutChangingFrameRate(t *testing.T) {
	for _, test := range []struct {
		name       string
		start, end time.Duration
		positions  map[time.Duration]time.Duration
	}{
		{"保持", 250, 250, map[time.Duration]time.Duration{0: 0, 200: 200, 250: 250, 500: 250, 20000: 250}},
		{"循环", 250, 650, map[time.Duration]time.Duration{0: 0, 249: 249, 250: 250, 649: 649, 650: 250, 900: 500, 10250: 250}},
		{"停末尾", 1000, 1000, map[time.Duration]time.Duration{999: 999, 1000: 1000, 2000: 1000}},
	} {
		t.Run(test.name, func(t *testing.T) {
			player := segmentedPlayback(t, test.start*time.Millisecond, test.end*time.Millisecond)
			for elapsed, position := range test.positions {
				if got := player.positionAt(elapsed * time.Millisecond); got != position*time.Millisecond {
					t.Errorf("经过 %dms：原时间轴 = %s，期望 %dms", elapsed, got, position)
				}
			}
			// 200ms 是第二帧起点，与不对齐帧边界的分段点无关。
			if player.frameAt(199*time.Millisecond) != 0 || player.frameAt(200*time.Millisecond) != 1 {
				t.Fatal("分段不应修改 GIF 原始帧延迟")
			}
		})
	}
}

func TestSegmentedFinishFromEntranceHoldAndLoop(t *testing.T) {
	start := time.Unix(100, 0)
	for _, test := range []struct {
		name             string
		loopEnd, request time.Duration
		positions        map[time.Duration]time.Duration
	}{
		{"进入时退出", 650, 100, map[time.Duration]time.Duration{100: 100, 249: 249, 250: 650, 500: 900, 600: 1000}},
		{"循环中退出", 650, 500, map[time.Duration]time.Duration{500: 500, 649: 649, 650: 650, 900: 900, 1000: 1000}},
		{"多次循环后退出", 650, 1000, map[time.Duration]time.Duration{1000: 600, 1050: 650, 1400: 1000}},
		{"保持时退出", 250, 500, map[time.Duration]time.Duration{500: 250, 600: 350, 1249: 999, 1250: 1000}},
	} {
		t.Run(test.name, func(t *testing.T) {
			player := segmentedPlayback(t, 250*time.Millisecond, test.loopEnd*time.Millisecond)
			if _, err := player.advance(start); err != nil {
				t.Fatal(err)
			}
			if err := player.finish(start.Add(test.request*time.Millisecond), func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			for elapsed, position := range test.positions {
				if got := player.positionAt(elapsed * time.Millisecond); got != position*time.Millisecond {
					t.Errorf("经过 %dms：回正位置 = %s，期望 %dms", elapsed, got, position)
				}
			}
		})
	}
}

func TestSegmentedFinishBeforeFirstFrameAndEmptyReturn(t *testing.T) {
	start := time.Unix(100, 0)
	for _, point := range []time.Duration{0, time.Second} {
		player := segmentedPlayback(t, point, point)
		called := 0
		if err := player.finish(start, func() error { called++; return nil }); err != nil {
			t.Fatal(err)
		}
		if err := player.finish(start, func() error { return nil }); err == nil {
			t.Fatal("同次播放不能重复请求回正")
		}
		for _, now := range []time.Time{start, start.Add(time.Second), start.Add(2 * time.Second)} {
			if _, err := player.advance(now); err != nil {
				t.Fatal(err)
			}
			if callback := player.takeCompletion(); callback != nil {
				if err := callback(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if called != 1 || player.current != 3 {
			t.Fatalf("完成后应保持末帧且只回调一次：called=%d frame=%d", called, player.current)
		}
	}
}

func TestSegmentedReturnCallbackSwitchesInSameFrame(t *testing.T) {
	for _, nextFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "目标在后", true: "目标在前"}[nextFirst], func(t *testing.T) {
			old, next := gifImageNode(t), gifImageNode(t)
			old.ID, old.Size = "old", image.Pt(4, 2)
			old.Image.Animation.Segments = &ui.GIFSegments{}
			next.ID, next.Visible = "next", false
			children := []*ui.Node{old, next}
			if nextFirst {
				children = []*ui.Node{next, old}
			}
			renderer := rendererFor(t, children...)
			frameFor(t, renderer, 8, 8)
			now := time.Now()
			called := 0
			if err := renderer.imagePlayers[old].finish(now, func() error {
				called++
				if err := renderer.SetVisible("old", false); err != nil {
					return err
				}
				return renderer.SetVisible("next", true)
			}); err != nil {
				t.Fatal(err)
			}
			if err := renderer.advanceImageAnimations(renderer.layout, now.Add(2*time.Hour)); err != nil {
				t.Fatal(err)
			}
			if called != 1 || renderer.imagePlayers[next].current != 0 || renderer.imagePlayers[old].decoder != nil {
				t.Fatal("旧 GIF 未停止，或目标首帧没有在切换的同一帧准备好")
			}
			frame := frameFor(t, renderer, 8, 8)
			pixel(t, frame, 0, 0, color.RGBA{R: 1, A: 255})
			pixel(t, frame, 3, 0, black)
		})
	}
}

func TestFinishImageRejectsWrongOrHiddenTargetsAndCancelsOnHide(t *testing.T) {
	node := gifImageNode(t)
	node.ID = "gif"
	node.Image.Animation.Segments = &ui.GIFSegments{}
	parent := block(0, 0, 4, 4, color.NRGBA{}, node)
	parent.ID = "group"
	renderer := rendererFor(t, parent)
	for _, id := range []string{"missing", "group"} {
		if err := renderer.FinishImage(id, func() error { return nil }); err == nil {
			t.Fatalf("错误目标 %s 应报错", id)
		}
	}
	if err := renderer.FinishImage("gif", nil); err == nil {
		t.Fatal("缺少回调应报错")
	}
	if err := renderer.SetVisible("group", false); err != nil {
		t.Fatal(err)
	}
	if err := renderer.FinishImage("gif", func() error { return nil }); err == nil || !strings.Contains(err.Error(), "hidden") {
		t.Fatalf("隐藏父节点应拒绝回正：%v", err)
	}
	if err := renderer.SetVisible("group", true); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	called := false
	if err := renderer.FinishImage("gif", func() error { called = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := renderer.SetVisible("group", false); err != nil {
		t.Fatal(err)
	}
	if renderer.imagePlayers[node].exit != nil {
		t.Fatal("隐藏子树未取消回正回调")
	}
	if err := renderer.SetVisible("group", true); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	if called || renderer.imagePlayers[node].current != 0 {
		t.Fatal("显示后应重新播放，不应调用旧回调")
	}
}

func TestFinishCallbackErrorReachesFrameLoop(t *testing.T) {
	node := gifImageNode(t)
	node.ID = "gif"
	node.Image.Animation.Segments = &ui.GIFSegments{}
	renderer := rendererFor(t, node)
	frameFor(t, renderer, 8, 8)
	failure := errors.New("回调失败")
	if err := renderer.FinishImage("gif", func() error { return failure }); err != nil {
		t.Fatal(err)
	}
	err := renderer.advanceImageAnimations(renderer.layout, time.Now().Add(3*time.Hour))
	if !errors.Is(err, failure) {
		t.Fatalf("完成回调错误未传回窗口循环：%v", err)
	}
}

func TestFinishAtFinalHoldCompletesWithoutFrameChange(t *testing.T) {
	start := time.Unix(100, 0)
	player := segmentedPlayback(t, time.Second, time.Second)
	if _, err := player.advance(start); err != nil {
		t.Fatal(err)
	}
	if _, err := player.advance(start.Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := player.finish(start.Add(2*time.Second), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	changed, err := player.advance(start.Add(2 * time.Second))
	if err != nil || changed || player.takeCompletion() == nil {
		t.Fatalf("末尾保持没有回正段，也应在下一帧完成：changed=%v err=%v", changed, err)
	}
}

func TestHiddenGIFCancelsAlreadyCollectedCompletion(t *testing.T) {
	first, second := gifImageNode(t), gifImageNode(t)
	first.ID, second.ID = "first", "second"
	first.Image.Animation.Segments = &ui.GIFSegments{}
	second.Image.Animation.Segments = &ui.GIFSegments{}
	renderer := rendererFor(t, first, second)
	frameFor(t, renderer, 8, 8)
	called := false
	if err := renderer.FinishImage("first", func() error {
		return renderer.SetVisible("second", false)
	}); err != nil {
		t.Fatal(err)
	}
	if err := renderer.FinishImage("second", func() error { called = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := renderer.advanceImageAnimations(renderer.layout, time.Now().Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("同帧其它回调隐藏图片后，不应再执行它已取消的完成回调")
	}
}

func TestAutoGIFRejectsFinish(t *testing.T) {
	node := gifImageNode(t)
	node.ID = "auto"
	renderer := rendererFor(t, node)
	if err := renderer.FinishImage("auto", func() error { return nil }); err == nil {
		t.Fatal("普通 GIF 不能调用分段回正")
	}
}
