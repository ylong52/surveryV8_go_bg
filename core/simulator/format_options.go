package simulator

// format_options.go — 选项列表水平格式化
//
// 规则：
//   - 水平排列，选项间 3 个空格
//   - 超出 maxWidth 时自动换行
//   - 每行开头对齐

import (
	"fmt"
	"strings"
)

// DefaultMaxWidth 默认终端宽度
const DefaultMaxWidth = 76

// DefaultSpacing 默认间距（空格数）
const DefaultSpacing = 3

// FormatOptionsHorizontal 将选项水平排列，超出宽度时自动换行
//
// @param options  选项列表
// @param maxWidth 终端最大宽度（默认 76）
// @param spacing  选项间距（默认 3 个空格）
// @returns       格式化后的行列表
func FormatOptionsHorizontal(options []SimOption, maxWidth int, spacing int) []string {
	if maxWidth <= 0 {
		maxWidth = DefaultMaxWidth
	}
	if spacing < 0 {
		spacing = DefaultSpacing
	}
	if len(options) == 0 {
		return nil
	}

	var lines []string
	var currentLine strings.Builder
	indent := "      " // 6 个空格缩进

	for i, opt := range options {
		item := fmt.Sprintf("[ %d ] %s", opt.Index, opt.Label)
		itemWithSpacing := item + strings.Repeat(" ", spacing)

		if currentLine.Len() == 0 {
			currentLine.WriteString(indent)
			currentLine.WriteString(item)
		} else if currentLine.Len()+len(itemWithSpacing) > maxWidth {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(indent)
			currentLine.WriteString(item)
		} else {
			currentLine.WriteString(spacingStr(spacing))
			currentLine.WriteString(item)
		}

		if i == len(options)-1 && currentLine.Len() > 0 {
			lines = append(lines, currentLine.String())
		}
	}

	return lines
}

func spacingStr(n int) string {
	return strings.Repeat(" ", n)
}

// FormatOptionsCompact 压缩格式：1,Fe  2,M  3,Other（取 label 前2个字符）
func FormatOptionsCompact(options []SimOption) string {
	if len(options) == 0 {
		return ""
	}
	var parts []string
	for _, opt := range options {
		label := opt.Label
		if len(label) > 2 {
			label = label[:2]
		}
		parts = append(parts, fmt.Sprintf("%d,%s", opt.Index, label))
	}
	return strings.Join(parts, "  ")
}
