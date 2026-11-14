package voice

import (
	"encoding/binary"
	"os"
	"path/filepath"
)

// encodeWAV 在 PCM 前加标准 WAV 文件头，便于接入识别服务或本地检查录音。
func encodeWAV(pcm []byte) []byte {
	data := make([]byte, 44+len(pcm))
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:16], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1) // PCM。
	binary.LittleEndian.PutUint16(data[22:24], 1) // 单声道。
	binary.LittleEndian.PutUint32(data[24:28], SampleRate)
	binary.LittleEndian.PutUint32(data[28:32], SampleRate*bytesPerSample)
	binary.LittleEndian.PutUint16(data[32:34], bytesPerSample)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(pcm)))
	copy(data[44:], pcm)
	return data
}

// SaveRecording 只保留最近一次录音，不在项目资源目录积累音频文件。
// 缓存仅用于排查；识别直接使用 Record 返回的内存数据。
func SaveRecording(data []byte) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(cache, "GoUI", "robot-face")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	filename := filepath.Join(directory, "last-recording.wav")
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		return "", err
	}
	return filename, nil
}
