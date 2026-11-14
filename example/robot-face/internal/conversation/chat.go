package conversation

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const maxReplyCharacters = 8192

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type completion struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message      Message `json:"message"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat 只读取正文 content，供应商的 reasoning_content 不显示也不朗读。
// emit 接收新增文字；返回错误或取消 context 会终止正在生成的回答。
func (c *Client) Chat(ctx context.Context, messages []Message, emit func(string) error) (string, error) {
	body := map[string]any{"model": c.config.LLM.Model, "messages": messages, "stream": true, "max_tokens": 512}
	resp, err := c.request(ctx, c.config.LLM.BaseURL+"/chat/completions", c.config.LLM.APIKey, "application/json", jsonBody(body))
	if err != nil {
		return "", fmt.Errorf("大模型: %w", err)
	}
	defer resp.Body.Close()
	var answer strings.Builder
	appendText := func(text string) error {
		if text == "" {
			return nil
		}
		if !utf8.ValidString(text) || utf8.RuneCountInString(answer.String())+utf8.RuneCountInString(text) > maxReplyCharacters {
			return errors.New("大模型回答超过长度限制或包含无效文字")
		}
		answer.WriteString(text)
		return emit(text)
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		err = readCompletionStream(resp.Body, appendText)
	} else {
		// 少数兼容服务忽略 stream，仍允许它返回一次完整回答。
		var data []byte
		data, err = readLimited(resp.Body, 128<<10)
		if err == nil {
			var result completion
			if err = json.Unmarshal(data, &result); err == nil {
				if result.Error != nil {
					err = errors.New("大模型返回错误响应")
				} else if len(result.Choices) > 0 {
					err = appendText(result.Choices[0].Message.Content)
				}
			}
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("读取模型回答: %w", err)
	}
	result := strings.TrimSpace(answer.String())
	if result == "" {
		return "", errors.New("大模型没有返回回答正文")
	}
	return result, nil
}

// readCompletionStream 按 SSE 事件读取，支持心跳和跨行 data，不把断流当成功。
func readCompletionStream(reader io.Reader, emit func(string) error) error {
	scanner := bufio.NewScanner(io.LimitReader(reader, 4<<20))
	scanner.Buffer(make([]byte, 4096), 256<<10)
	var data []string
	finished := false
	flush := func() (bool, error) {
		if len(data) == 0 {
			return false, nil
		}
		value := strings.Join(data, "\n")
		data = nil
		if value == "[DONE]" {
			return true, nil
		}
		var result completion
		if err := json.Unmarshal([]byte(value), &result); err != nil {
			return false, errors.New("模型返回了无效的流式数据")
		}
		if result.Error != nil {
			return false, errors.New("大模型返回流式错误")
		}
		if len(result.Choices) > 0 {
			choice := result.Choices[0]
			if err := emit(choice.Delta.Content); err != nil {
				return false, err
			}
			finished = choice.FinishReason != nil && *choice.FinishReason != ""
		}
		return false, nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			done, err := flush()
			if err != nil || done {
				return err
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if done, err := flush(); err != nil || done {
		return err
	}
	if !finished {
		return io.ErrUnexpectedEOF
	}
	return nil
}
