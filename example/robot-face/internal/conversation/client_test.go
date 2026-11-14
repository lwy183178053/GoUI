package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewClient(Config{
		Speech: SpeechConfig{URL: server.URL, Voice: "demo0001", APIKey: "speech-secret"},
		LLM:    LLMConfig{BaseURL: server.URL + "/v1", Model: "test-model", APIKey: "llm-secret", SystemPrompt: "简短回答"},
	})
}

func TestSpeechRequests(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer speech-secret" {
			t.Error("语音请求的方法或认证错误")
		}
		switch r.URL.Path {
		case "/v1/audio/transcriptions":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				http.Error(w, "bad form", 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			if r.FormValue("model") != "whisper-1" || r.FormValue("language") != "zh" || r.FormValue("response_format") != "json" {
				t.Error("识别参数错误")
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			data, _ := io.ReadAll(file)
			if string(data) != "test-wav" {
				t.Error("录音上传内容错误")
			}
			io.WriteString(w, `{"text":"  你好机器人  "}`)
		case "/v1/audio/speech":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			if body["input"] != "你好。" || body["voice"] != "demo0001" || body["response_format"] != "wav" || body["language"] != "zh" {
				t.Errorf("合成参数错误: %v", body)
			}
			w.Write([]byte("synthesized-wav"))
		default:
			http.NotFound(w, r)
		}
	})
	text, err := client.Transcribe(context.Background(), []byte("test-wav"))
	if err != nil || text != "你好机器人" {
		t.Fatalf("识别: %q, %v", text, err)
	}
	wav, err := client.Synthesize(context.Background(), "你好。")
	if err != nil || string(wav) != "synthesized-wav" {
		t.Fatalf("合成: %q, %v", wav, err)
	}
}

func TestChatAcceptsJSONFallback(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer llm-secret" {
			t.Error("模型地址或认证错误")
		}
		var body struct {
			Model    string
			Stream   bool
			Messages []Message
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "test-model" || !body.Stream || len(body.Messages) != 1 || body.Messages[0].Content != "问题" {
			t.Error("模型请求错误")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"回答。"}}]}`)
	})
	var delta string
	text, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "问题"}}, func(value string) error { delta += value; return nil })
	if err != nil || text != "回答。" || delta != text {
		t.Fatalf("JSON 回答: %q, %q, %v", text, delta, err)
	}
}

func TestClientErrorsAndCancellation(t *testing.T) {
	t.Run("redact-key", func(t *testing.T) {
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"error":{"message":"invalid llm-secret"}}`)
		})
		_, err := client.Synthesize(context.Background(), "测试")
		if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "llm-secret") {
			t.Fatalf("错误提示不正确: %v", err)
		}
	})
	t.Run("empty-transcript", func(t *testing.T) {
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"text":" "}`) })
		if _, err := client.Transcribe(context.Background(), []byte("audio")); err == nil {
			t.Fatal("空识别应提示重试")
		}
	})
	t.Run("cancel-stream", func(t *testing.T) {
		started, stopped := make(chan struct{}), make(chan struct{})
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, ": connected\n\n")
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
			close(stopped)
		})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() { _, err := client.Chat(ctx, nil, func(string) error { return nil }); done <- err }()
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("请求未启动")
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("取消失败: %v", err)
		}
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("取消后服务端请求仍未结束")
		}
	})
}
