package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func sessionClient(t *testing.T, deltas []string, requests *[][]Message, requestMu *sync.Mutex) *Client {
	t.Helper()
	return testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/audio/transcriptions":
			io.WriteString(w, `{"text":"你好"}`)
		case "/v1/chat/completions":
			var body struct{ Messages []Message }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			requestMu.Lock()
			*requests = append(*requests, body.Messages)
			requestMu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			for _, delta := range deltas {
				data, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": delta}}}})
				io.WriteString(w, "data: "+string(data)+"\n\n")
				w.(http.Flusher).Flush()
			}
			io.WriteString(w, "data: [DONE]\n\n")
		case "/v1/audio/speech":
			var body struct{ Input string }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			w.Write([]byte(body.Input))
		default:
			http.NotFound(w, r)
		}
	})
}

func TestSessionStreamsSentencesAndKeepsSuccessfulHistory(t *testing.T) {
	var mu sync.Mutex
	var requests [][]Message
	session := NewSession(sessionClient(t, []string{"你好", "。今天", "很好！", "再见"}, &requests, &mu))
	var played []string
	var updates []Update
	emit := func(update Update) error { mu.Lock(); defer mu.Unlock(); updates = append(updates, update); return nil }
	play := func(_ context.Context, wav []byte) error { played = append(played, string(wav)); return nil }
	for range 7 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := session.Respond(ctx, []byte("audio"), emit, play)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(played[:3], []string{"你好。", "今天很好！", "再见"}) || len(played) != 21 {
		t.Fatalf("播放顺序错误: %v", played)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(session.history) != 12 || len(requests[1]) != 4 || len(requests[6]) != 14 {
		t.Fatalf("对话历史错误: history=%d requests=%v", len(session.history), requests)
	}
	lastText := ""
	for _, update := range updates {
		if update.Text != "" {
			lastText = update.Text
		}
	}
	if lastText != "你好。今天很好！再见" {
		t.Fatalf("字幕不完整: %q", lastText)
	}
}

func TestSessionInterruptionCancelsPlaybackAndDoesNotSaveHistory(t *testing.T) {
	var mu sync.Mutex
	var requests [][]Message
	session := NewSession(sessionClient(t, []string{"第一句。第二句。第三句。"}, &requests, &mu))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	calls := 0
	go func() {
		done <- session.Respond(ctx, []byte("audio"), func(Update) error { return ctx.Err() }, func(playCtx context.Context, _ []byte) error {
			calls++
			close(started)
			<-playCtx.Done()
			return playCtx.Err()
		})
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("播放未开始")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("打断结果错误: %v", err)
	}
	if calls != 1 || len(session.history) != 0 {
		t.Fatal("打断后不能继续播放队列，也不能保存半轮对话")
	}
}

func TestSessionPlaybackErrorCancelsTurn(t *testing.T) {
	var mu sync.Mutex
	var requests [][]Message
	session := NewSession(sessionClient(t, []string{"回答。"}, &requests, &mu))
	err := session.Respond(context.Background(), []byte("audio"), func(Update) error { return nil }, func(context.Context, []byte) error { return errors.New("扬声器不可用") })
	if err == nil || !strings.Contains(err.Error(), "扬声器不可用") || len(session.history) != 0 {
		t.Fatalf("播放失败未正确结束对话: %v", err)
	}
}

func TestSplitSentence(t *testing.T) {
	for _, value := range []string{"没有标点", "第一句。第二句", "你好!继续", strings.Repeat("字", 121)} {
		sentence, rest := splitSentence(value)
		if sentence+rest != value {
			t.Fatal("切句丢失文字")
		}
		if value == "没有标点" && sentence != "" {
			t.Fatal("短句应继续等待正文")
		}
		if strings.HasPrefix(value, "字") && len([]rune(sentence)) != 120 {
			t.Fatal("长句应分段")
		}
	}
}
