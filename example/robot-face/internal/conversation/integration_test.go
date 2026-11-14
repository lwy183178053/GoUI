package conversation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"example.com/goui-robot-face/internal/voice"
)

// 本机服务验收需显式指定配置和测试音频；常规测试不会联网或打开扬声器。
func TestLocalConversationIntegration(t *testing.T) {
	configPath, audioPath := os.Getenv("ROBOT_INTEGRATION_CONFIG"), os.Getenv("ROBOT_INTEGRATION_WAV")
	if configPath == "" || audioPath == "" {
		t.Skip("设置 ROBOT_INTEGRATION_CONFIG 和 ROBOT_INTEGRATION_WAV 后测试真实服务")
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	session := NewSession(NewClient(cfg))
	if err := session.Validate(); err != nil {
		t.Fatal(err)
	}
	wav, err := os.ReadFile(audioPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	start := time.Now()
	count := 0
	err = session.Respond(ctx, wav, func(update Update) error {
		t.Logf("%s: %s (%d 字)", time.Since(start).Round(time.Millisecond), update.Phase, len([]rune(update.Text)))
		return ctx.Err()
	}, func(ctx context.Context, data []byte) error {
		count++
		if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
			return fmt.Errorf("第 %d 句没有返回 WAV 音频", count)
		}
		if output := os.Getenv("ROBOT_INTEGRATION_OUTPUT"); output != "" {
			if err := os.MkdirAll(output, 0700); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(output, fmt.Sprintf("reply-%02d.wav", count)), data, 0600); err != nil {
				return err
			}
		}
		if os.Getenv("ROBOT_INTEGRATION_PLAY") == "1" {
			return voice.Play(ctx, data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("没有产生回答语音")
	}
	t.Logf("识别、流式回答、合成完成，共 %d 段，耗时 %s", count, time.Since(start).Round(time.Millisecond))
}
