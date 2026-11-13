package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/goui-robot-face/internal/robot"
	"example.com/goui-robot-face/internal/testui"
	goui "github.com/lwy183178053/GoUI"
)

func testController(t *testing.T) (*robot.Controller, string) {
	t.Helper()
	app, directory, dispatch := testui.Open(t)
	controller, err := robot.New(app.Page(), dispatch)
	if err != nil {
		t.Fatal(err)
	}
	return controller, directory
}

func TestEmotionHTTPAPI(t *testing.T) {
	controller, directory := testController(t)
	router := New(controller, directory)
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
			Current     robot.EmotionState `json:"current"`
			Expressions []robot.Expression `json:"expressions"`
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
		if response.Code != http.StatusOK || controller.Emotion().Target.Name != "happy" || !controller.Emotion().Transitioning {
			t.Fatalf("切换失败：%d, %s", response.Code, response.Body.String())
		}
	})
	t.Run("整数零可以使用", func(t *testing.T) {
		response := request(http.MethodPost, "/api/emotion", `{"valence":0,"arousal":0}`)
		if response.Code != http.StatusOK || controller.Emotion().Vector != (robot.Vector{Valence: 0, Arousal: 0}) {
			t.Fatalf("零值被拒绝：%d, %s", response.Code, response.Body.String())
		}
	})
	t.Run("拒绝非法输入且保留状态", func(t *testing.T) {
		before := controller.Emotion()
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
			if controller.Emotion() != before {
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
		app, directory, _ := testui.Open(t)
		for _, test := range []struct {
			err    error
			status int
		}{
			{context.Canceled, http.StatusServiceUnavailable},
			{context.DeadlineExceeded, http.StatusGatewayTimeout},
		} {
			controller, err := robot.New(app.Page(), func(context.Context, func(*goui.Page) error) error { return test.err })
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/emotion", strings.NewReader(`{"valence":1,"arousal":9}`))
			New(controller, directory).ServeHTTP(response, req)
			if response.Code != test.status {
				t.Fatalf("窗口错误状态码 = %d，期望 %d", response.Code, test.status)
			}
		}
	})
}
