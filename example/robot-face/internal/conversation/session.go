package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type Phase string

const (
	Recognizing Phase = "recognizing"
	Thinking    Phase = "thinking"
	Answering   Phase = "answering"
	Speaking    Phase = "speaking"
)

// Update 只携带业务状态和当前完整文字；界面更新由机器人控制器负责。
// Text 为空表示只改变状态，保留当前字幕。
type Update struct {
	Phase Phase
	Text  string
}

// Session 由一个对话任务顺序使用，保存最近六轮成功完成的对话。
// 被打断或失败的半句不写入历史，也不在磁盘保存聊天内容。
type Session struct {
	client  *Client
	history []Message
}

func NewSession(client *Client) *Session { return &Session{client: client} }
func (s *Session) Validate() error       { return s.client.Validate() }

// Respond 串起识别、模型、按句合成和顺序播放。三个流水线阶段共用取消信号，
// 任一阶段失败或用户打断，网络请求、待播队列和当前播放一起停止。
func (s *Session) Respond(ctx context.Context, wav []byte, emit func(Update) error, play func(context.Context, []byte) error) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := emit(Update{Phase: Recognizing, Text: "正在识别…"}); err != nil {
		return err
	}
	text, err := s.client.Transcribe(ctx, wav)
	if err != nil {
		return err
	}
	if err := emit(Update{Phase: Thinking, Text: "你：" + text + "\n正在思考…"}); err != nil {
		return err
	}
	messages := []Message{{Role: "system", Content: s.client.config.LLM.SystemPrompt}}
	messages = append(messages, s.history...)
	messages = append(messages, Message{Role: "user", Content: text})

	group, taskCtx := errgroup.WithContext(ctx)
	sentences := make(chan string, 2)
	audio := make(chan []byte, 2)
	var answer string
	group.Go(func() error {
		defer close(sentences)
		var full strings.Builder
		pending := ""
		send := func(sentence string) error {
			if strings.TrimSpace(sentence) == "" {
				return nil
			}
			select {
			case sentences <- sentence:
				return nil
			case <-taskCtx.Done():
				return taskCtx.Err()
			}
		}
		var err error
		answer, err = s.client.Chat(taskCtx, messages, func(delta string) error {
			full.WriteString(delta)
			if err := emit(Update{Phase: Answering, Text: full.String()}); err != nil {
				return err
			}
			pending += delta
			for {
				sentence, rest := splitSentence(pending)
				if sentence == "" {
					break
				}
				if err := send(sentence); err != nil {
					return err
				}
				pending = rest
			}
			return nil
		})
		if err != nil {
			return err
		}
		return send(pending)
	})
	group.Go(func() error {
		defer close(audio)
		for {
			select {
			case <-taskCtx.Done():
				return taskCtx.Err()
			case sentence, ok := <-sentences:
				if !ok {
					return nil
				}
				wav, err := s.client.Synthesize(taskCtx, sentence)
				if err != nil {
					return err
				}
				select {
				case audio <- wav:
				case <-taskCtx.Done():
					return taskCtx.Err()
				}
			}
		}
	})
	group.Go(func() error {
		for {
			select {
			case <-taskCtx.Done():
				return taskCtx.Err()
			case wav, ok := <-audio:
				if !ok {
					return nil
				}
				if err := emit(Update{Phase: Speaking}); err != nil {
					return err
				}
				if err := play(taskCtx, wav); err != nil {
					return fmt.Errorf("播放语音: %w", err)
				}
			}
		}
	})
	if err := group.Wait(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.history = append(s.history, Message{Role: "user", Content: text}, Message{Role: "assistant", Content: answer})
	if len(s.history) > 12 {
		s.history = append([]Message(nil), s.history[len(s.history)-12:]...)
	}
	return nil
}

// splitSentence 优先按句末标点切分；过长的单句每 120 字切一段，避免首段等太久。
func splitSentence(value string) (sentence, rest string) {
	count := 0
	for index, r := range value {
		count++
		if strings.ContainsRune("。！？!?；;\n", r) || count >= 120 {
			end := index + len(string(r))
			return value[:end], value[end:]
		}
	}
	return "", value
}
