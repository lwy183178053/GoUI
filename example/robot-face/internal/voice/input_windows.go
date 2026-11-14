package voice

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 通过系统 WinMM 录音，不依赖 CGO。四个 20ms 缓冲轮流交给系统填充。
const (
	inputBufferCount = 4
	inputBufferBytes = SampleRate * bytesPerSample / 50
	waveHeaderDone   = 1
)

var (
	winmm                 = windows.NewLazySystemDLL("winmm.dll")
	waveInOpen            = winmm.NewProc("waveInOpen")
	waveInPrepareHeader   = winmm.NewProc("waveInPrepareHeader")
	waveInAddBuffer       = winmm.NewProc("waveInAddBuffer")
	waveInStart           = winmm.NewProc("waveInStart")
	waveInReset           = winmm.NewProc("waveInReset")
	waveInUnprepareHeader = winmm.NewProc("waveInUnprepareHeader")
	waveInClose           = winmm.NewProc("waveInClose")
	waveInGetErrorText    = winmm.NewProc("waveInGetErrorTextW")
)

// 字段顺序与 Windows 的 WAVEFORMATEX、WAVEHDR 保持一致。
type waveFormat struct {
	format           uint16
	channels         uint16
	samplesPerSecond uint32
	bytesPerSecond   uint32
	blockAlign       uint16
	bitsPerSample    uint16
	extraSize        uint16
}

type waveHeader struct {
	data     uintptr
	length   uint32
	recorded uint32
	user     uintptr
	flags    uint32
	loops    uint32
	next     uintptr
	reserved uintptr
}

type windowsInput struct {
	ctx      context.Context
	handle   uintptr
	headers  []waveHeader
	buffers  [][]byte
	pinned   runtime.Pinner
	prepared int
	current  int
	offset   int
}

func waveError(operation string, code uintptr) error {
	if code == 0 {
		return nil
	}
	var message [256]uint16
	waveInGetErrorText.Call(code, uintptr(unsafe.Pointer(&message[0])), uintptr(len(message)))
	return fmt.Errorf("%s: %s (WinMM %d)", operation, windows.UTF16ToString(message[:]), code)
}

func openInput(ctx context.Context) (io.ReadCloser, error) {
	input := &windowsInput{ctx: ctx}
	format := waveFormat{
		format: 1, channels: 1, samplesPerSecond: SampleRate,
		bytesPerSecond: SampleRate * bytesPerSample,
		blockAlign:     bytesPerSample, bitsPerSample: 16,
	}
	// WAVE_MAPPER 使用系统默认麦克风；缓冲完成由 Read 检查，无需本地回调。
	code, _, _ := waveInOpen.Call(uintptr(unsafe.Pointer(&input.handle)), 0xffffffff,
		uintptr(unsafe.Pointer(&format)), 0, 0, 0)
	if err := waveError("打开默认麦克风", code); err != nil {
		return nil, err
	}
	input.headers = make([]waveHeader, inputBufferCount)
	input.buffers = make([][]byte, inputBufferCount)
	input.pinned.Pin(&input.headers[0])
	for i := range input.headers {
		input.buffers[i] = make([]byte, inputBufferBytes)
		input.pinned.Pin(&input.buffers[i][0])
		header := &input.headers[i]
		header.data = uintptr(unsafe.Pointer(&input.buffers[i][0]))
		header.length = inputBufferBytes
		code, _, _ = waveInPrepareHeader.Call(input.handle,
			uintptr(unsafe.Pointer(header)), unsafe.Sizeof(*header))
		if err := waveError("准备录音缓冲", code); err != nil {
			input.Close()
			return nil, err
		}
		input.prepared++
		if err := input.queue(header); err != nil {
			input.Close()
			return nil, err
		}
	}
	code, _, _ = waveInStart.Call(input.handle)
	if err := waveError("开始录音", code); err != nil {
		input.Close()
		return nil, err
	}
	return input, nil
}

func (input *windowsInput) queue(header *waveHeader) error {
	code, _, _ := waveInAddBuffer.Call(input.handle,
		uintptr(unsafe.Pointer(header)), unsafe.Sizeof(*header))
	return waveError("提交录音缓冲", code)
}

func (input *windowsInput) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	for {
		if err := input.ctx.Err(); err != nil {
			return 0, err
		}
		header := &input.headers[input.current]
		if atomic.LoadUint32(&header.flags)&waveHeaderDone != 0 {
			if header.recorded > header.length {
				return 0, fmt.Errorf("麦克风返回了超出缓冲长度的音频")
			}
			n := copy(buffer, input.buffers[input.current][input.offset:header.recorded])
			input.offset += n
			if input.offset == int(header.recorded) {
				if err := input.queue(header); err != nil {
					return n, err
				}
				input.current = (input.current + 1) % inputBufferCount
				input.offset = 0
			}
			if n > 0 {
				return n, nil
			}
		}
		select {
		case <-input.ctx.Done():
			return 0, input.ctx.Err()
		case <-timer.C:
			timer.Reset(10 * time.Millisecond)
		}
	}
}

func (input *windowsInput) Close() error {
	if input.handle == 0 {
		return nil
	}
	// 先让系统归还所有缓冲，再释放固定内存，避免系统继续写入已释放的地址。
	waveInReset.Call(input.handle)
	for i := 0; i < input.prepared; i++ {
		header := &input.headers[i]
		waveInUnprepareHeader.Call(input.handle,
			uintptr(unsafe.Pointer(header)), unsafe.Sizeof(*header))
	}
	code, _, _ := waveInClose.Call(input.handle)
	input.handle = 0
	input.pinned.Unpin()
	return waveError("关闭麦克风", code)
}
