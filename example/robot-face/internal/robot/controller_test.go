package robot

import (
	"os"
	"path/filepath"
	"testing"

	"example.com/goui-robot-face/internal/testui"
)

func testRobot(t *testing.T) (*Controller, string) {
	t.Helper()
	app, directory, dispatch := testui.Open(t)
	controller, err := New(app.Page(), dispatch)
	if err != nil {
		t.Fatal(err)
	}
	return controller, directory
}

func TestRobotStartsWithNormalExpression(t *testing.T) {
	robot, _ := testRobot(t)
	got := robot.Emotion()
	if got.Expression.Name != "normal" || got.Loop || got.Pending != nil {
		t.Fatalf("初始状态不正确：%+v", got)
	}
}

func TestDownloadedResourcesMatchExpressions(t *testing.T) {
	gifFiles, err := filepath.Glob("../../ui/gif/*.gif")
	if err != nil {
		t.Fatal(err)
	}
	if len(gifFiles) != len(expressions) {
		t.Fatalf("有 %d 张 GIF，登记了 %d 张", len(gifFiles), len(expressions))
	}
	for _, item := range expressions {
		if _, err := os.Stat(filepath.Join("..", "..", "ui", "gif", item.File)); err != nil {
			t.Errorf("表情 %s 的素材不存在：%v", item.Name, err)
		}
	}
}
