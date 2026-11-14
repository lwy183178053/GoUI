package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"testing"
	"time"
)

func TestDecodeWAVWithExtraChunks(t *testing.T) {
	pcm := []byte{0, 0, 1, 0, 2, 0}
	original := encodeWAV(pcm)
	// JUNK 长度为奇数，后面还带一个对齐字节。
	data := append([]byte(nil), original[:36]...)
	data = append(data, []byte{'J', 'U', 'N', 'K', 3, 0, 0, 0, 1, 2, 3, 0}...)
	data = append(data, original[36:]...)
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	audio, err := decodeWAV(data)
	if err != nil {
		t.Fatal(err)
	}
	if audio.rate != SampleRate || audio.channels != 1 || !bytes.Equal(audio.samples, pcm) {
		t.Fatal("WAV 解码改变了采样信息或内容")
	}
}

func TestDecodeWAVRejectsBrokenAudio(t *testing.T) {
	valid := encodeWAV([]byte{0, 0, 1, 0})
	for _, name := range []string{"not-wav", "truncated", "compressed", "float32", "channels", "rate", "unaligned", "no-data", "chunk-length"} {
		t.Run(name, func(t *testing.T) {
			data := bytes.Clone(valid)
			switch name {
			case "not-wav":
				data = []byte("service error")
			case "truncated":
				data = data[:len(data)-1]
			case "compressed":
				binary.LittleEndian.PutUint16(data[20:22], 3)
			case "float32":
				binary.LittleEndian.PutUint16(data[34:36], 32)
			case "channels":
				binary.LittleEndian.PutUint16(data[22:24], 0)
			case "rate":
				binary.LittleEndian.PutUint32(data[24:28], 0)
			case "unaligned":
				binary.LittleEndian.PutUint32(data[40:44], 3)
			case "no-data":
				copy(data[36:40], "JUNK")
			case "chunk-length":
				binary.LittleEndian.PutUint32(data[40:44], 0xffffffff)
			}
			if _, err := decodeWAV(data); err == nil {
				t.Fatal("损坏音频不能交给系统播放")
			}
		})
	}
}

func TestPlayCanceledBeforeOpeningDevice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Play(ctx, encodeWAV([]byte{0, 0})); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消结果错误: %v", err)
	}
}

// 显式设置测试音频后才打开真实设备，验证取消会归还播放缓冲。
func TestPlaybackDeviceCancellation(t *testing.T) {
	path := os.Getenv("ROBOT_TEST_PLAYBACK_WAV")
	if path == "" {
		t.Skip("设置 ROBOT_TEST_PLAYBACK_WAV 后验证真实扬声器取消")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	audio, err := decodeWAV(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(audio.samples) < int(audio.rate)*int(audio.channels)*2 {
		t.Fatal("测试音频至少需要一秒")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := Play(ctx, data); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("设备播放取消失败: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("取消后播放设备没有及时归还")
	}
}
