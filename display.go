package goui

import "github.com/lwy183178053/GoUI/internal/backend/window"

// Display 描述桌面系统检测到的一个显示器。索引从 0 开始，主显示器排在第一位。
type Display struct {
	Index  int
	Name   string
	Width  int
	Height int
}

// Displays 返回当前桌面系统检测到的显示器。
func Displays() ([]Display, error) {
	items, err := window.Displays()
	if err != nil {
		return nil, err
	}
	result := make([]Display, len(items))
	for i, item := range items {
		result[i] = Display{
			Index: item.Index, Name: item.Name, Width: item.Width, Height: item.Height,
		}
	}
	return result, nil
}
