package voice

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestRecordingReportsReadyOnceAndKeepsCompleteSamples(t *testing.T) {
	ready := 0
	input := bytes.NewReader(bytes.Repeat([]byte{1}, 6401))
	pcm, err := readPCM(context.Background(), input, func() { ready++ })
	if err != nil || len(pcm) != 6400 || ready != 1 {
		t.Fatalf("应只通知一次开始，并去掉末尾的半个采样: %d %d %v", len(pcm), ready, err)
	}
}

func TestCancelledRecordingRetainsCapturedAudio(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pcm, err := readPCM(ctx, bytes.NewReader([]byte{1, 2, 3, 4}), cancel)
	if err != nil || !bytes.Equal(pcm, []byte{1, 2, 3, 4}) {
		t.Fatalf("手动停止应保留已录音频: %v %v", pcm, err)
	}
	if _, err := readPCM(ctx, bytes.NewReader(nil), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("尚未录到音频就取消时应返回取消原因: %v", err)
	}
}

func TestRecordingLimitAndDeviceFailures(t *testing.T) {
	pcm, err := readPCM(context.Background(), io.LimitReader(zeroReader{}, int64(maxPCMBytes)+100), nil)
	if err != nil || len(pcm) != maxPCMBytes {
		t.Fatalf("录音必须有明确长度上限: %d %v", len(pcm), err)
	}
	if _, err := readPCM(context.Background(), bytes.NewReader(nil), nil); err == nil {
		t.Fatal("设备未提供音频时应报错")
	}
	failure := errors.New("设备断开")
	if _, err := readPCM(context.Background(), failedReader{failure}, nil); !errors.Is(err, failure) {
		t.Fatalf("应保留设备错误: %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type failedReader struct{ err error }

func (r failedReader) Read([]byte) (int, error) { return 0, r.err }

// 预先取消时不能打开真实设备，测试也不会访问麦克风。
func TestRecordAlreadyCancelledDoesNotOpenDevice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	audio, err := Record(ctx, func() { t.Error("取消后不应开始录音") })
	if !errors.Is(err, context.Canceled) || len(audio) != 0 {
		t.Fatalf("取消后应直接返回，不打开设备: %v %v", audio, err)
	}
}
