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
	controller := testController(t)
	router := New(controller, testSnapshot)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		return response
	}
	for _, value := range []string{"正在倾听…", "你好，这是完整回答。", "好", ""} {
		payload, err := json.Marshal(map[string]string{"text": value})
		if err != nil {
			t.Fatal(err)
		}
		response := request(http.MethodPost, "/api/display", string(payload))
		var body struct {
			Current robot.DisplayState `json:"current"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		characters := []rune(value)
		want := robot.DisplayState{Text: value, VisibleText: string(characters[:min(1, len(characters))]), Typing: len(characters) > 1}
		if response.Code != http.StatusOK || body.Current != want || controller.Display() != want {
			t.Fatalf("文字应立即接收全文并从首字开始：%d %s", response.Code, response.Body.String())
		}
	}
	response := request(http.MethodGet, "/api/display", "")
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("读取显示状态失败或允许了缓存")
	}
	before := controller.Display()
	for _, body := range []string{
		"", "null", "{}", "[]", `{"text":null}`, `{"text":3}`, `{"append":"旧增量接口"}`,
		`{"text":"a","append":"b"}`, `{"status":"listening"}`, `{"text":"a","status":"idle"}`, `{"extra":1}`,
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
