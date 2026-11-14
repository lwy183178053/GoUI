package conversation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"ROBOT_SPEECH_URL", "ROBOT_SPEECH_KEY", "ROBOT_VOICE", "ROBOT_LLM_URL", "ROBOT_LLM_KEY", "ROBOT_LLM_MODEL"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfigDefaultsAndEnvironment(t *testing.T) {
	clearConfigEnv(t)
	missing := filepath.Join(t.TempDir(), "missing.json")
	cfg, err := LoadConfig(missing)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Speech.Voice != "demo0001" || cfg.Speech.URL != "http://127.0.0.1:3900" {
		t.Fatalf("语音默认值错误: %+v", cfg.Speech)
	}
	if cfg.Validate() == nil {
		t.Fatal("模型未配置时应提示缺少配置")
	}
	t.Setenv("ROBOT_LLM_URL", "https://provider.example/v1/")
	t.Setenv("ROBOT_LLM_MODEL", "test-model")
	t.Setenv("ROBOT_LLM_KEY", "environment-key")
	cfg, err = LoadConfig(missing)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.BaseURL != "https://provider.example/v1" || cfg.LLM.APIKey != "environment-key" {
		t.Fatal("环境变量覆盖或地址规范化失败")
	}
}

func TestConfigRejectsInvalidFiles(t *testing.T) {
	clearConfigEnv(t)
	for name, content := range map[string]string{
		"unknown-field":      `{"unknown":true}`,
		"multiple-objects":   `{} {}`,
		"invalid-json":       `{`,
		"invalid-url":        `{"speech":{"url":"file:///tmp/service"}}`,
		"credentials-in-url": `{"speech":{"url":"https://user:password@example.com"}}`,
		"oversize":           `{"llm":{"system_prompt":"` + strings.Repeat("a", 64<<10) + `"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "voice.json")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(path); err == nil {
				t.Fatal("应拒绝非法配置")
			}
		})
	}
}
