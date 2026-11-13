package robot

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	goui "github.com/lwy183178053/GoUI"
)

// ErrInvalidInput 让 HTTP 层区分业务输入错误与窗口、资源等运行错误。
var ErrInvalidInput = errors.New("输入无效")

// MaxTextLength 限制单次提交的字符数。
const MaxTextLength = 8192

// textCharacterInterval 控制打字机速度，中文和英文都按字符显示。
const textCharacterInterval = 40 * time.Millisecond

// DisplayState 区分收到的完整句子和设备上已经显示的部分。
type DisplayState struct {
	Text        string `json:"text"`
	VisibleText string `json:"visible_text"`
	Typing      bool   `json:"typing"`
}

// typewriter 只记录当前句子的播放进度，不为每句话启动后台任务。
type typewriter struct {
	characters []rune
	started    time.Time
	shown      int
}

// Display 返回文字状态的快照。
func (c *Controller) Display() DisplayState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.display
}

// SetDisplay 一次接收完整句子。新句子替换旧内容，空字符串立即清空。
// 首字立即显示，其余字符由 RunEffects 通过窗口主循环逐步显示。
func (c *Controller) SetDisplay(ctx context.Context, value string) (DisplayState, error) {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > MaxTextLength {
		return DisplayState{}, fmt.Errorf("%w: text 必须是有效 UTF-8，不超过 %d 个字符", ErrInvalidInput, MaxTextLength)
	}
	characters := []rune(value)
	var result DisplayState
	err := c.dispatch(ctx, func(*goui.Page) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		shown := min(1, len(characters))
		visible := string(characters[:shown])
		if err := c.caption.SetText(visible); err != nil {
			return err
		}
		if err := c.caption.SetVisible(visible != ""); err != nil {
			return err
		}
		c.typing = typewriter{characters: characters, started: time.Now(), shown: shown}
		c.display = DisplayState{Text: value, VisibleText: visible, Typing: shown < len(characters)}
		c.wakeEffects()
		result = c.display
		return nil
	})
	return result, err
}

// nextTextTime 在持有 mu 时读取，完成或清空后不再安排更新。
func (c *Controller) nextTextTime() time.Time {
	if !c.display.Typing {
		return time.Time{}
	}
	return c.typing.started.Add(time.Duration(c.typing.shown) * textCharacterInterval)
}

// advanceText 在主循环且持有 mu 时调用，始终读取最新句子。
// 按已过时间计算进度，窗口短暂忙碌也不会让整句话越来越慢。
func (c *Controller) advanceText(now time.Time) error {
	if !c.display.Typing {
		return nil
	}
	elapsed := max(0, now.Sub(c.typing.started))
	shown := min(len(c.typing.characters), 1+int(elapsed/textCharacterInterval))
	if shown <= c.typing.shown {
		return nil
	}
	visible := string(c.typing.characters[:shown])
	if err := c.caption.SetText(visible); err != nil {
		return err
	}
	c.typing.shown = shown
	c.display.VisibleText = visible
	c.display.Typing = shown < len(c.typing.characters)
	return nil
}
