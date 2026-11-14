// Package voice 负责机器人项目的麦克风录音，不依赖 GoUI 窗口和组件。
package voice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	SampleRate     = 16000
	bytesPerSample = 2 // 单声道、16 位、小端 PCM。
	MaxDuration    = time.Minute
	maxPCMBytes    = SampleRate * bytesPerSample * int(MaxDuration/time.Second)
)

// Record 从默认麦克风录音。收到第一段音频才调用 started；取消 ctx 即结束。
// 单次最多一分钟。正常停止返回已录到的 WAV，设备错误则返回具体原因。
func Record(ctx context.Context, started func()) ([]byte, error) {
	// 截止时间和数据量都在这里限制，调用方无需另设一分钟定时器。
	ctx, cancel := context.WithTimeout(ctx, MaxDuration)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input, err := openInput(ctx)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	pcm, err := readPCM(ctx, input, started)
	if err != nil {
		return nil, err
	}
	return encodeWAV(pcm), nil
}

func readPCM(ctx context.Context, input io.Reader, started func()) ([]byte, error) {
	pcm := make([]byte, 0, SampleRate*bytesPerSample)
	chunk := make([]byte, 3200)
	for len(pcm) < maxPCMBytes {
		if ctx.Err() != nil {
			break
		}
		n, err := input.Read(chunk[:min(len(chunk), maxPCMBytes-len(pcm))])
		if n > 0 {
			pcm = append(pcm, chunk[:n]...)
			if started != nil {
				started()
				started = nil
			}
		}
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("读取麦克风: %w", err)
		}
	}
	// 一个采样占两个字节，取消时可能只读到半个采样。
	pcm = pcm[:len(pcm)/bytesPerSample*bytesPerSample]
	if len(pcm) == 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("麦克风没有返回音频，请检查默认录音设备和麦克风权限")
	}
	return pcm, nil
}
