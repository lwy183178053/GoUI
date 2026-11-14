package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
)

// Transcribe 复用已经可用的上传识别接口，不依赖实时听写模型的许可状态。
func (c *Client) Transcribe(ctx context.Context, wav []byte) (string, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	audio, err := form.CreateFormFile("file", "recording.wav")
	if err != nil {
		return "", err
	}
	if _, err = audio.Write(wav); err != nil {
		return "", err
	}
	for key, value := range map[string]string{"model": "whisper-1", "language": "zh", "response_format": "json"} {
		if err := form.WriteField(key, value); err != nil {
			return "", err
		}
	}
	if err := form.Close(); err != nil {
		return "", err
	}
	resp, err := c.request(ctx, c.config.Speech.URL+"/v1/audio/transcriptions", c.config.Speech.APIKey, form.FormDataContentType(), &body)
	if err != nil {
		return "", fmt.Errorf("语音识别: %w", err)
	}
	defer resp.Body.Close()
	data, err := readLimited(resp.Body, 64<<10)
	if err != nil {
		return "", err
	}
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("解析识别结果: %w", err)
	}
	text := strings.TrimSpace(result.Text)
	if text == "" {
		return "", errors.New("没有识别到说话内容，请靠近麦克风再试一次")
	}
	if len([]rune(text)) > maxReplyCharacters {
		return "", errors.New("识别文字过长")
	}
	return text, nil
}

// Synthesize 每次只合成一个句子，明确指定带参考音频的 VoiceStudio 音色。
func (c *Client) Synthesize(ctx context.Context, text string) ([]byte, error) {
	body := map[string]any{"model": "tts-1", "input": text, "voice": c.config.Speech.Voice, "response_format": "wav", "language": "zh"}
	resp, err := c.request(ctx, c.config.Speech.URL+"/v1/audio/speech", c.config.Speech.APIKey, "application/json", jsonBody(body))
	if err != nil {
		return nil, fmt.Errorf("语音合成: %w", err)
	}
	defer resp.Body.Close()
	return readLimited(resp.Body, 16<<20)
}
