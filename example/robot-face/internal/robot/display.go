package robot

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	goui "github.com/lwy183178053/GoUI"
)

// ErrInvalidInput 让 HTTP 层区分业务输入错误与窗口、资源等运行错误。
var ErrInvalidInput = errors.New("输入无效")

// MaxTextLength 限制累计显示字符数，避免持续追加无界占用内存。
const MaxTextLength = 8192

// DisplayState 保存唯一文字区域的内容，和表情向量相互独立。
type DisplayState struct {
	Text string `json:"text"`
}

// DisplayUpdate 只修改提供的字段。Text 替换全文，Append 追加增量，两者不能同时使用。
// 指针用于区分遗漏字段和空字符串，空 text 可以清空文字。
type DisplayUpdate struct {
	Text   *string `json:"text"`
	Append *string `json:"append"`
}

// Validate 检查字段组合；追加后的累计长度在主循环中检查。
func (update DisplayUpdate) Validate() error {
	if update.Text == nil && update.Append == nil {
		return fmt.Errorf("%w: 至少提供 text 或 append", ErrInvalidInput)
	}
	if update.Text != nil && update.Append != nil {
		return fmt.Errorf("%w: text 与 append 不能同时提供", ErrInvalidInput)
	}
	return nil
}

// Display 返回文字状态的快照。
func (c *Controller) Display() DisplayState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.display
}

// SetDisplay 在主循环中更新普通 Text；HTTP 流式输出逐段提交 Append 即可。
func (c *Controller) SetDisplay(ctx context.Context, update DisplayUpdate) (DisplayState, error) {
	if err := update.Validate(); err != nil {
		return DisplayState{}, err
	}
	var result DisplayState
	err := c.dispatch(ctx, func(*goui.Page) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		next := c.display
		if update.Text != nil {
			next.Text = *update.Text
		}
		if update.Append != nil {
			next.Text += *update.Append
		}
		if !utf8.ValidString(next.Text) || utf8.RuneCountInString(next.Text) > MaxTextLength {
			return fmt.Errorf("%w: text 必须是有效 UTF-8，累计不超过 %d 个字符", ErrInvalidInput, MaxTextLength)
		}
		if err := c.caption.SetText(next.Text); err != nil {
			return err
		}
		if err := c.caption.SetVisible(next.Text != ""); err != nil {
			return err
		}
		c.display = next
		result = next
		return nil
	})
	if err != nil {
		return DisplayState{}, err
	}
	return result, nil
}
