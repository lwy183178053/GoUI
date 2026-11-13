package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/goui-robot-face/internal/robot"
)

func TestDisplayHTTPAPI(t *testing.T) {
	controller, directory := testController(t)
	router := New(controller, directory)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		return response
	}
	for _, test := range []struct {
		body         string
		text, status string
	}{
		{`{"text":"你好","status":"speaking"}`, "你好", "speaking"},
		{`{"append":"，这是实时回答。"}`, "你好，这是实时回答。", "speaking"},
		{`{"status":"listening"}`, "你好，这是实时回答。", "listening"},
		{`{"text":"","status":"idle"}`, "", "idle"},
	} {
		response := request(http.MethodPost, "/api/display", test.body)
		var body struct {
			Current robot.DisplayState `json:"current"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || body.Current.Text != test.text || body.Current.Status != test.status {
			t.Fatalf("显示接口返回不正确：%d %s", response.Code, response.Body.String())
		}
	}
	response := request(http.MethodGet, "/api/display", "")
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("读取显示状态失败或允许了缓存")
	}
	before := controller.Display()
	for _, body := range []string{
		"", "null", "{}", "[]", `{"text":3}`, `{"append":true}`,
		`{"text":"a","append":"b"}`, `{"status":"unknown"}`, `{"extra":1}`,
		`{"text":"a"} {}`, `{"text":"` + strings.Repeat("字", robot.MaxTextLength+1) + `"}`,
		`{"text":"x"}` + strings.Repeat(" ", 64*1024),
	} {
		response := request(http.MethodPost, "/api/display", body)
		if response.Code != http.StatusBadRequest || controller.Display() != before {
			t.Fatalf("非法请求应失败且保留状态：%d %s", response.Code, response.Body.String())
		}
	}
	if response := request(http.MethodGet, "/display.js", ""); response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatal("文字调试脚本没有发布")
	}
}
