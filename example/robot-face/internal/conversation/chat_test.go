package conversation

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadCompletionStream(t *testing.T) {
	cases := []struct {
		name, data, want string
		wantError        bool
	}{
		{name: "chinese-and-heartbeat", data: ": ping\n\ndata: {\"choices\":[{\"delta\":{\"reasoning_content\":\"hidden\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"。\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", want: "你好。"},
		{name: "multiline-data", data: "data: {\"choices\":[\ndata: {\"delta\":{\"content\":\"好\"}}]}\n\ndata: [DONE]\n\n", want: "好"},
		{name: "finish-without-done", data: `data: {"choices":[{"delta":{"content":"完成"},"finish_reason":"stop"}]}`, want: "完成"},
		{name: "unexpected-eof", data: `data: {"choices":[{"delta":{"content":"半句"}}]}`, want: "半句", wantError: true},
		{name: "invalid-json", data: "data: broken\n\n", wantError: true},
		{name: "service-error", data: `data: {"error":{"message":"failed"}}`, wantError: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var result strings.Builder
			err := readCompletionStream(strings.NewReader(test.data), func(delta string) error {
				result.WriteString(delta)
				return nil
			})
			if (err != nil) != test.wantError || result.String() != test.want {
				t.Fatalf("got %q, %v; want %q, error=%v", result.String(), err, test.want, test.wantError)
			}
			if test.name == "unexpected-eof" && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("断流错误不明确: %v", err)
			}
		})
	}
}

func TestReadCompletionStreamStopsOnCallbackError(t *testing.T) {
	data := `data: {"choices":[{"delta":{"content":"取消"}}]}` + "\n\ndata: [DONE]\n\n"
	err := readCompletionStream(strings.NewReader(data), func(string) error { return context.Canceled })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("回调取消未传递: %v", err)
	}
}
