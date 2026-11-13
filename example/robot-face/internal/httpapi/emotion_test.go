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

func testController(t *testing.T) *robot.Controller {
	t.Helper()
	app, _, dispatch := testui.Open(t)
	controller, err := robot.New(app.Page(), dispatch)
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

func TestEmotionHTTPAPI(t *testing.T) {
	controller := testController(t)
	router := New(controller, testSnapshot)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	t.Run("读取表情列表", func(t *testing.T) {
		response := request(http.MethodGet, "/api/expressions", "")
		var body struct {
			Expressions []robot.Expression `json:"expressions"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(body.Expressions) != 23 || body.Expressions[0].Name != "normal" {
			t.Fatalf("表情列表不正确：%d, %s", response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), "valence") {
			t.Fatal("名称列表不应缓存或包含旧坐标")
		}
	})
	t.Run("读取初始状态", func(t *testing.T) {
		response := request(http.MethodGet, "/api/emotion", "")
		var body struct {
			Current robot.EmotionState `json:"current"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || body.Current.Expression.Name != "normal" || body.Current.Loop || body.Current.Pending != nil {
			t.Fatalf("初始状态不正确：%d, %s", response.Code, response.Body.String())
		}
	})
	t.Run("按名切换默认立即播放一次", func(t *testing.T) {
		response := request(http.MethodPost, "/api/emotion", `{"name":"happy"}`)
		if response.Code != http.StatusOK || controller.Emotion().Expression.Name != "happy" || controller.Emotion().Loop || controller.Emotion().Pending != nil {
			t.Fatalf("切换失败：%d, %s", response.Code, response.Body.String())
		}
	})
	t.Run("循环与等待本轮结束", func(t *testing.T) {
		response := request(http.MethodPost, "/api/emotion", `{"name":"music","loop":true}`)
		if response.Code != http.StatusOK || !controller.Emotion().Loop || controller.Emotion().Expression.Name != "music" {
			t.Fatalf("音乐应持续播放：%d, %s", response.Code, response.Body.String())
		}
		for _, name := range []string{"happy", "sad"} {
			response = request(http.MethodPost, "/api/emotion", `{"name":"`+name+`","interrupt":false}`)
			state := controller.Emotion()
			if response.Code != http.StatusOK || state.Expression.Name != "music" || state.Pending == nil || state.Pending.Name != name {
				t.Fatalf("应在音乐结束后切到最新目标：%d, %s", response.Code, response.Body.String())
			}
		}
		response = request(http.MethodPost, "/api/emotion", `{"name":"normal","loop":true,"interrupt":true}`)
		if response.Code != http.StatusOK || controller.Emotion().Loop || controller.Emotion().Expression.Name != "normal" || controller.Emotion().Pending != nil {
			t.Fatalf("应打断等待，立即回正常眨眼：%d, %s", response.Code, response.Body.String())
		}
	})
	t.Run("拒绝非法输入且保留状态", func(t *testing.T) {
		before := controller.Emotion()
		for _, body := range []string{
			"", "{}", "null", "[]", `{"name":null}`, `{"name":12}`,
			`{"name":"missing"}`, `{"name":"MUSIC"}`,
			`{"name":"music","loop":"true"}`, `{"name":"music","loop":1}`,
			`{"name":"music","interrupt":"false"}`, `{"name":"music","interrupt":0}`,
			`{"name":"music","unknown":true}`, `{"valence":3,"arousal":3}`,
			`{"name":"music"} {}`, `{"name":"music"}` + strings.Repeat(" ", 1100),
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
	t.Run("内嵌网页和禁止原始素材路径", func(t *testing.T) {
		for _, path := range []string{"/", "/styles.css", "/api.js", "/app.js", "/display.js", "/preview.js"} {
			response := request(http.MethodGet, path, "")
			if response.Code != http.StatusOK || response.Body.Len() == 0 {
				t.Errorf("静态文件 %s = %d", path, response.Code)
			}
		}
		for _, path := range []string{"/gif/smile_%5B0.5s%5D.gif", "/gif/missing.gif", "/gif/config.xml", "/config.xml", "/web/"} {
			if response := request(http.MethodGet, path, ""); response.Code != http.StatusNotFound {
				t.Errorf("未开放文件 %s = %d", path, response.Code)
			}
		}
	})
}

func TestEmotionHTTPWindowUnavailable(t *testing.T) {
	app, _, _ := testui.Open(t)
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
		req := httptest.NewRequest(http.MethodPost, "/api/emotion", strings.NewReader(`{"name":"angry"}`))
		New(controller, testSnapshot).ServeHTTP(response, req)
		if response.Code != test.status {
			t.Fatalf("窗口错误状态码 = %d，期望 %d", response.Code, test.status)
		}
	}
}
