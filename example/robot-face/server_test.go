package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	goui "github.com/lwy183178053/GoUI"
)

func TestEmotionHTTPAPI(t *testing.T) {
	robot, directory := testRobot(t)
	router := newRouter(robot, directory)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	t.Run("读取初始状态", func(t *testing.T) {
		response := request(http.MethodGet, "/api/emotion", "")
		var body struct {
			Current     emotionState `json:"current"`
			Expressions []expression `json:"expressions"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || body.Current.Expression.Name != "normal" || len(body.Expressions) != 23 {
			t.Fatalf("初始响应不正确：%d, %s", response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("状态接口不应被浏览器缓存")
		}
	})
	t.Run("向量切换", func(t *testing.T) {
		response := request(http.MethodPost, "/api/emotion", `{"valence":9,"arousal":9}`)
		if response.Code != http.StatusOK || robot.state().Target.Name != "happy" || !robot.state().Transitioning {
			t.Fatalf("切换失败：%d, %s", response.Code, response.Body.String())
		}
	})
	t.Run("整数零可以使用", func(t *testing.T) {
		response := request(http.MethodPost, "/api/emotion", `{"valence":0,"arousal":0}`)
		if response.Code != http.StatusOK || robot.state().Vector != (vector{0, 0}) {
			t.Fatalf("零值被拒绝：%d, %s", response.Code, response.Body.String())
		}
	})
	t.Run("拒绝非法输入且保留状态", func(t *testing.T) {
		before := robot.state()
		for _, body := range []string{
			"", "{}", "null", "[8,8]",
			`{"valence":8}`, `{"valence":null,"arousal":8}`,
			`{"valence":"8","arousal":8}`, `{"valence":8.5,"arousal":8}`,
			`{"valence":8,"arousal":8,"unknown":1}`,
			`{"valence":-1,"arousal":8}`, `{"valence":8,"arousal":11}`,
			`{"valence":8,"arousal":8} {}`,
			`{"valence":8,"arousal":8}` + strings.Repeat(" ", 1100),
		} {
			response := request(http.MethodPost, "/api/emotion", body)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "error") {
				t.Errorf("非法请求 %q = %d, %s", body, response.Code, response.Body.String())
			}
			if robot.state() != before {
				t.Fatal("非法请求修改了当前表情")
			}
		}
	})
	t.Run("内嵌网页和 GIF", func(t *testing.T) {
		for _, path := range []string{"/", "/styles.css", "/app.js", "/gif/smile_%5B0.5s%5D.gif"} {
			response := request(http.MethodGet, path, "")
			if response.Code != http.StatusOK || response.Body.Len() == 0 {
				t.Errorf("静态文件 %s = %d", path, response.Code)
			}
		}
		for _, path := range []string{"/gif/missing.gif", "/gif/config.xml", "/config.xml", "/web/"} {
			if response := request(http.MethodGet, path, ""); response.Code != http.StatusNotFound {
				t.Errorf("未开放文件 %s = %d", path, response.Code)
			}
		}
	})
	t.Run("窗口退出后报告不可用", func(t *testing.T) {
		previous := robot.dispatch
		defer func() { robot.dispatch = previous }()
		robot.dispatch = func(context.Context, func(*goui.Page) error) error {
			return context.Canceled
		}
		response := request(http.MethodPost, "/api/emotion", `{"valence":1,"arousal":9}`)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("窗口不可用时返回 %d", response.Code)
		}
		robot.dispatch = func(context.Context, func(*goui.Page) error) error {
			return context.DeadlineExceeded
		}
		response = request(http.MethodPost, "/api/emotion", `{"valence":1,"arousal":9}`)
		if response.Code != http.StatusGatewayTimeout {
			t.Fatalf("等待窗口超时时返回 %d", response.Code)
		}
	})
}
