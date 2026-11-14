package voice

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWAVContainsMono16BitPCM(t *testing.T) {
	pcm := []byte{0, 0, 1, 0, 255, 127, 0, 128}
	data := encodeWAV(pcm)
	if string(data[:4]) != "RIFF" || string(data[8:16]) != "WAVEfmt " || string(data[36:40]) != "data" {
		t.Fatal("WAV 文件头不正确")
	}
	if binary.LittleEndian.Uint32(data[4:8]) != uint32(len(data)-8) ||
		binary.LittleEndian.Uint32(data[24:28]) != SampleRate ||
		binary.LittleEndian.Uint16(data[22:24]) != 1 ||
		binary.LittleEndian.Uint16(data[34:36]) != 16 ||
		binary.LittleEndian.Uint32(data[40:44]) != uint32(len(pcm)) ||
		!bytes.Equal(data[44:], pcm) {
		t.Fatal("WAV 格式或音频内容不正确")
	}
}

func TestSaveRecordingReplacesOnlyLatestWAV(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("缓存路径按当前支持录音的 Windows 和 Linux 验证")
	}
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	expected := filepath.Join(cache, "GoUI", "robot-face", "last-recording.wav")
	for _, pcm := range [][]byte{{0, 0, 1, 0}, {2, 0}} {
		audio := encodeWAV(pcm)
		filename, err := SaveRecording(audio)
		if err != nil || filename != expected {
			t.Fatalf("最近录音的保存位置不正确: %q %v", filename, err)
		}
		data, err := os.ReadFile(filename)
		if err != nil || !bytes.Equal(data, audio) {
			t.Fatalf("应完整保存本次录音，覆盖旧录音: %v", err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(expected))
	if err != nil || len(entries) != 1 {
		t.Fatalf("不能积累多份历史录音: %v %v", entries, err)
	}
}
