package software

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lwy183178053/GoUI/internal/ui"
)

func TestGIFPlayOnceIgnoresFileLoopAndCompletesAfterFinalDelay(t *testing.T) {
	player := newImagePlayback(testGIFAnimation(t, 0, 100*time.Millisecond, 200*time.Millisecond))
	calls := 0
	player.play(false, func() error { calls++; return nil })
	start := time.Unix(100, 0)
	for _, elapsed := range []time.Duration{0, 100 * time.Millisecond, 299 * time.Millisecond} {
		if _, err := player.advance(start.Add(elapsed)); err != nil {
			t.Fatal(err)
		}
		if player.takeCompletion() != nil {
			t.Fatal("最后一帧的显示时长未结束，不能提前通知完成")
		}
	}
	if _, err := player.advance(start.Add(300 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	callback := player.takeCompletion()
	if callback == nil || callback() != nil || calls != 1 {
		t.Fatal("完整播放后应通知一次")
	}
	if _, err := player.advance(start.Add(time.Hour)); err != nil || player.current != 1 || player.takeCompletion() != nil {
		t.Fatal("完成后应保持末帧，且不重复通知")
	}
}

func TestGIFPlayLoopOverridesFiniteFileAndReplayCancelsOldCallback(t *testing.T) {
	player := newImagePlayback(testGIFAnimation(t, -1, 100*time.Millisecond, 200*time.Millisecond))
	start := time.Unix(100, 0)
	player.play(false, func() error { t.Error("旧完成通知不应执行"); return nil })
	if _, err := player.advance(start); err != nil {
		t.Fatal(err)
	}
	if _, err := player.advance(start.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	old := player.takeCompletion()
	if old == nil {
		t.Fatal("测试应已收集旧完成通知")
	}
	player.play(true, nil)
	if err := old(); err != nil {
		t.Fatal(err)
	}
	for _, elapsed := range []time.Duration{0, time.Second, 2 * time.Second} {
		if _, err := player.advance(start.Add(elapsed)); err != nil {
			t.Fatal(err)
		}
		if player.positionAt(elapsed) != elapsed%player.duration || player.takeCompletion() != nil {
			t.Fatal("显式循环应覆盖 GIF 的一次播放设置")
		}
	}
}

func TestPlayCallbackPreparesReplacementFirstFrameAndReportsErrors(t *testing.T) {
	renderer, doc, _ := sourceRenderer(t)
	node := doc.Find("face")
	node.Image.Playback, node.Image.Animation.Segments = "auto", nil
	calls := 0
	if err := renderer.PlayImage("face", false, func() error {
		calls++
		return renderer.SetImageSource("face", "next_[0.1s].gif")
	}); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	if err := renderer.advanceImageAnimations(renderer.layout, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	player := renderer.imagePlayers[node]
	if calls != 1 || player.current != 0 || player.frame() == nil || node.Image.Source != "next_[0.1s].gif" {
		t.Fatal("完成回调换源后，应在同一帧准备好新图片")
	}
	failure := errors.New("业务处理失败")
	if err := renderer.PlayImage("face", false, func() error { return failure }); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	err := renderer.advanceImageAnimations(renderer.layout, time.Now().Add(time.Second))
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "main.xml:") {
		t.Fatalf("完成通知应保留业务错误与 XML 来源：%v", err)
	}
}

func TestPlayRejectsInvalidTargetsAndDoesNotInterruptCurrentPlayback(t *testing.T) {
	node := gifImageNode(t)
	node.ID = "face"
	renderer := rendererFor(t, node)
	if err := renderer.PlayImage("face", false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	player := renderer.imagePlayers[node]
	before := player.completion
	if err := renderer.PlayImage("face", true, func() error { return nil }); err == nil || player.completion != before {
		t.Fatal("循环不能声明完成通知，失败时保留原播放")
	}
	if err := renderer.PlayImage("missing", false, nil); err == nil {
		t.Fatal("缺少 ID 应报错")
	}
	if err := renderer.SetVisible("face", false); err != nil {
		t.Fatal(err)
	}
	if err := renderer.PlayImage("face", false, nil); err == nil {
		t.Fatal("隐藏的 GIF 不能播放")
	}
	node.Visible = true
	player.animation.Segments = &ui.GIFSegments{}
	if err := renderer.PlayImage("face", false, nil); err == nil {
		t.Fatal("分段 GIF 继续使用 Finish，不能混用 Play")
	}
	delete(renderer.imagePlayers, node)
	if err := renderer.PlayImage("face", false, nil); err == nil {
		t.Fatal("静态图片不能播放 GIF")
	}
}

func TestPlayCancelsAnotherAlreadyCollectedCallback(t *testing.T) {
	first, second := gifImageNode(t), gifImageNode(t)
	first.ID, second.ID = "first", "second"
	renderer := rendererFor(t, first, second)
	if err := renderer.PlayImage("first", false, func() error {
		return renderer.PlayImage("second", true, nil)
	}); err != nil {
		t.Fatal(err)
	}
	if err := renderer.PlayImage("second", false, func() error {
		t.Error("已重播的 GIF 不应执行旧通知")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	if err := renderer.advanceImageAnimations(renderer.layout, time.Now().Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestHideCancelsOncePlaybackCompletion(t *testing.T) {
	node := gifImageNode(t)
	node.ID = "face"
	renderer := rendererFor(t, node)
	if err := renderer.PlayImage("face", false, func() error {
		t.Error("隐藏后不应执行旧通知")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	frameFor(t, renderer, 8, 8)
	if err := renderer.SetVisible("face", false); err != nil {
		t.Fatal(err)
	}
	if err := renderer.SetVisible("face", true); err != nil {
		t.Fatal(err)
	}
	if err := renderer.advanceImageAnimations(renderer.layout, time.Now().Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestFinishFullGIFCompletesCurrentRoundWithoutRestart(t *testing.T) {
	for _, mode := range []string{"file", "loop", "once"} {
		t.Run(mode, func(t *testing.T) {
			player := newImagePlayback(testGIFAnimation(t, 0, 100*time.Millisecond, 200*time.Millisecond, 200*time.Millisecond))
			if mode != "file" {
				player.play(mode == "loop", nil)
			}
			start := time.Unix(100, 0)
			if _, err := player.advance(start); err != nil {
				t.Fatal(err)
			}
			elapsed := 350 * time.Millisecond
			if mode != "once" {
				elapsed += 500 * time.Millisecond // 循环模式测试第二轮中途结束。
			}
			if _, err := player.advance(start.Add(elapsed)); err != nil {
				t.Fatal(err)
			}
			called := false
			if err := player.finish(start.Add(elapsed), func() error { called = true; return nil }); err != nil {
				t.Fatal(err)
			}
			if player.positionAt(elapsed) != 350*time.Millisecond {
				t.Fatal("请求完成本轮不应改变播放位置")
			}
			if _, err := player.advance(start.Add(elapsed + 149*time.Millisecond)); err != nil || player.takeCompletion() != nil {
				t.Fatalf("距离本轮末尾还有 1ms，不应通知完成：%v", err)
			}
			if _, err := player.advance(start.Add(elapsed + 150*time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			callback := player.takeCompletion()
			if callback == nil || callback() != nil || !called {
				t.Fatal("应在本轮末尾完成，不等待完整新一轮")
			}
		})
	}
}
