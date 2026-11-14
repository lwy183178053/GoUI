package voice

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"
)

var (
	waveOutOpen            = winmm.NewProc("waveOutOpen")
	waveOutPrepareHeader   = winmm.NewProc("waveOutPrepareHeader")
	waveOutWrite           = winmm.NewProc("waveOutWrite")
	waveOutReset           = winmm.NewProc("waveOutReset")
	waveOutUnprepareHeader = winmm.NewProc("waveOutUnprepareHeader")
	waveOutClose           = winmm.NewProc("waveOutClose")
)

// Windows 使用和录音相同的 WinMM 接口；数据播放期间固定内存，不依赖 CGO。
func playAudio(ctx context.Context, _ []byte, audio pcmAudio) error {
	format := waveFormat{format: 1, channels: audio.channels, samplesPerSecond: audio.rate,
		bytesPerSecond: audio.rate * uint32(audio.channels) * 2, blockAlign: audio.channels * 2, bitsPerSample: 16}
	var handle uintptr
	code, _, _ := waveOutOpen.Call(uintptr(unsafe.Pointer(&handle)), 0xffffffff, uintptr(unsafe.Pointer(&format)), 0, 0, 0)
	if err := waveError("打开默认扬声器", code); err != nil {
		return err
	}
	defer waveOutClose.Call(handle)
	header := waveHeader{data: uintptr(unsafe.Pointer(&audio.samples[0])), length: uint32(len(audio.samples))}
	var pinned runtime.Pinner
	pinned.Pin(&audio.samples[0])
	pinned.Pin(&header)
	defer pinned.Unpin()
	size := unsafe.Sizeof(header)
	code, _, _ = waveOutPrepareHeader.Call(handle, uintptr(unsafe.Pointer(&header)), size)
	if err := waveError("准备播放缓冲", code); err != nil {
		return err
	}
	defer waveOutUnprepareHeader.Call(handle, uintptr(unsafe.Pointer(&header)), size)
	// 先 reset 再 unprepare，保证取消时系统已经归还缓冲。
	defer waveOutReset.Call(handle)
	code, _, _ = waveOutWrite.Call(handle, uintptr(unsafe.Pointer(&header)), size)
	if err := waveError("提交播放音频", code); err != nil {
		return err
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if atomic.LoadUint32(&header.flags)&waveHeaderDone != 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
