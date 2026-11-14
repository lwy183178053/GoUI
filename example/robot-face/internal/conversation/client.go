package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 的请求都绑定当前对话的 context；打断后立刻停止等待网络结果。
type Client struct {
	config Config
	http   *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{config: cfg, http: &http.Client{
		Timeout: 2 * time.Minute,
		// 服务地址由用户显式指定，不带着凭据跟随重定向。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *Client) Validate() error { return c.config.Validate() }

func (c *Client) request(ctx context.Context, endpoint, key, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("连接服务失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		detail := ""
		var problem struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Detail json.RawMessage `json:"detail"`
		}
		if json.Unmarshal(data, &problem) == nil {
			detail = problem.Error.Message
			if detail == "" {
				_ = json.Unmarshal(problem.Detail, &detail)
			}
		}
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		// 原样显示服务的必要错误，但不把配置里的 Key 写到日志或页面。
		for _, secret := range []string{c.config.LLM.APIKey, c.config.Speech.APIKey} {
			if secret != "" {
				detail = strings.ReplaceAll(detail, secret, "[已隐藏]")
			}
		}
		return nil, fmt.Errorf("服务返回 HTTP %d: %s", resp.StatusCode, detail)
	}
	return resp, nil
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("服务返回内容超过大小限制")
	}
	return data, nil
}

func jsonBody(value any) io.Reader {
	data, _ := json.Marshal(value)
	return bytes.NewReader(data)
}
