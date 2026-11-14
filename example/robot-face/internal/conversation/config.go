// Package conversation 负责机器人项目的语音服务和对话，不依赖 GoUI。
package conversation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

// Config 是机器人自己的服务配置，与 ui/config.xml 的窗口配置分开。
// API Key 可以从本地配置读取，也可以用环境变量覆盖。
type Config struct {
	Speech SpeechConfig `json:"speech"`
	LLM    LLMConfig    `json:"llm"`
}

type SpeechConfig struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key,omitempty"`
	Voice  string `json:"voice"`
}

type LLMConfig struct {
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key,omitempty"`
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
}

// LoadConfig 缺少本地配置时仍允许打开界面；点击录音后会给出配置提示。
func LoadConfig(filename string) (Config, error) {
	cfg := Config{
		Speech: SpeechConfig{URL: "http://127.0.0.1:3900", Voice: "demo0001"},
		LLM:    LLMConfig{SystemPrompt: "你是一个友善的桌面机器人。用简洁、自然的中文回答，通常不超过三句话。回答会直接朗读，请使用普通文本，不要 Markdown、表格或思考过程。"},
	}
	file, err := os.Open(filename)
	if err == nil {
		defer file.Close()
		decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			return cfg, fmt.Errorf("读取语音配置 %s: %w", filename, err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return cfg, errors.New("语音配置只能包含一个 JSON 对象")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, fmt.Errorf("读取语音配置: %w", err)
	}
	for key, target := range map[string]*string{
		"ROBOT_SPEECH_URL": &cfg.Speech.URL, "ROBOT_SPEECH_KEY": &cfg.Speech.APIKey,
		"ROBOT_VOICE": &cfg.Speech.Voice, "ROBOT_LLM_URL": &cfg.LLM.BaseURL,
		"ROBOT_LLM_KEY": &cfg.LLM.APIKey, "ROBOT_LLM_MODEL": &cfg.LLM.Model,
	} {
		if value, ok := os.LookupEnv(key); ok {
			*target = strings.TrimSpace(value)
		}
	}
	if cfg.Speech.URL, err = cleanURL(cfg.Speech.URL); err != nil {
		return cfg, fmt.Errorf("speech.url: %w", err)
	}
	if cfg.LLM.BaseURL != "" {
		if cfg.LLM.BaseURL, err = cleanURL(cfg.LLM.BaseURL); err != nil {
			return cfg, fmt.Errorf("llm.base_url: %w", err)
		}
	}
	return cfg, nil
}

func cleanURL(value string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("需要 http/https 服务地址，不能含账号、查询参数或片段")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// Validate 在录音前检查必需项，避免说完话后才发现没有配置模型。
func (cfg Config) Validate() error {
	if cfg.LLM.BaseURL == "" || cfg.LLM.Model == "" {
		return errors.New("请配置 voice.local.json 中的 llm.base_url 和 llm.model，或设置 ROBOT_LLM_URL、ROBOT_LLM_MODEL")
	}
	if cfg.Speech.Voice == "" {
		return errors.New("请配置 speech.voice，GPT-SoVITS 需要带参考音频的音色 ID")
	}
	return nil
}
