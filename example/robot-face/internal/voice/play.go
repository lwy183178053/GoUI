package voice

import (
	"context"
	"encoding/binary"
	"errors"
)

type pcmAudio struct {
	channels uint16
	rate     uint32
	samples  []byte
}

// Play 播放服务返回的 PCM WAV；取消 ctx 会立即停止并释放默认播放设备。
func Play(ctx context.Context, wav []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	audio, err := decodeWAV(wav)
	if err != nil {
		return err
	}
	return playAudio(ctx, wav, audio)
}

// decodeWAV 按区块读取，允许 fmt/data 之间带 LIST、JUNK 等额外信息。
// 只接收本项目合成接口使用的 16 位 PCM，不把损坏文件交给系统驱动。
func decodeWAV(data []byte) (pcmAudio, error) {
	var audio pcmAudio
	invalid := errors.New("语音服务没有返回有效的 16 位 PCM WAV")
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return audio, invalid
	}
	end := uint64(binary.LittleEndian.Uint32(data[4:8])) + 8
	if end > uint64(len(data)) || end < 12 {
		return audio, invalid
	}
	data = data[:end]
	haveFormat := false
	for offset := 12; offset+8 <= len(data); {
		length := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		start := offset + 8
		if length > uint64(len(data)-start) {
			return audio, invalid
		}
		chunk := data[start : start+int(length)]
		switch string(data[offset : offset+4]) {
		case "fmt ":
			if len(chunk) < 16 || binary.LittleEndian.Uint16(chunk[:2]) != 1 || binary.LittleEndian.Uint16(chunk[14:16]) != 16 {
				return audio, invalid
			}
			audio.channels = binary.LittleEndian.Uint16(chunk[2:4])
			audio.rate = binary.LittleEndian.Uint32(chunk[4:8])
			if audio.channels < 1 || audio.channels > 2 || audio.rate < 8000 || audio.rate > 192000 || binary.LittleEndian.Uint16(chunk[12:14]) != audio.channels*2 {
				return audio, invalid
			}
			haveFormat = true
		case "data":
			audio.samples = chunk
		}
		offset = start + int(length) + int(length%2)
	}
	if !haveFormat || len(audio.samples) == 0 || len(audio.samples)%int(audio.channels*2) != 0 {
		return audio, invalid
	}
	return audio, nil
}
