package software

import (
	"image/color"
	"testing"
)

func TestSnapshotIsIndependentAndDoesNotRepaint(t *testing.T) {
	node := block(0, 0, 4, 4, red)
	node.ID = "block"
	renderer := rendererFor(t, node)
	if _, err := renderer.Snapshot(); err == nil {
		t.Fatal("绘制前没有可捕获的画面")
	}
	frameFor(t, renderer, 4, 4)
	snapshot, err := renderer.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.SetColor("block", blue); err != nil {
		t.Fatal(err)
	}
	// 修改组件不应让截图自行重绘，仍然只返回上一帧。
	previous, err := renderer.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	pixel(t, previous, 1, 1, color.RGBA{R: 255, A: 255})
	frameFor(t, renderer, 4, 4)
	pixel(t, snapshot, 1, 1, color.RGBA{R: 255, A: 255})
	snapshot.SetRGBA(1, 1, color.RGBA{})
	pixel(t, renderer.frame, 1, 1, color.RGBA{B: 255, A: 255})
	if err := renderer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := renderer.Snapshot(); err == nil {
		t.Fatal("关闭后不应允许捕获")
	}
}
