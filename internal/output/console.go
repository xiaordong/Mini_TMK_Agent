// Package output 提供输出抽象，支持控制台、TTS 等多种输出方式
package output

import (
	"fmt"
	"strings"
)

// ANSI 颜色常量
const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorBold   = "\033[1m"
)

// Output 输出接口，方便扩展 TTS、WebUI 等
type Output interface {
	// OnSourceText 显示原文
	OnSourceText(text string)
	// OnTranslatedText 显示译文（流式，每次追加一个片段）
	OnTranslatedText(chunk string)
	// OnTranslationEnd 翻译完成
	OnTranslationEnd()
	// OnInfo 显示提示信息
	OnInfo(msg string)
	// OnError 显示错误
	OnError(msg string)
}

// ConsoleOutput 控制台输出实现
type ConsoleOutput struct {
	verbose bool // 是否同时显示原文
}

// NewConsoleOutput 创建控制台输出
func NewConsoleOutput(verbose bool) *ConsoleOutput {
	return &ConsoleOutput{verbose: verbose}
}

func (o *ConsoleOutput) OnSourceText(text string) {
	if o.verbose {
		fmt.Printf("%s[原文] %s%s\n", colorGray, text, colorReset)
	}
}

func (o *ConsoleOutput) OnTranslatedText(chunk string) {
	fmt.Printf("%s%s%s", colorCyan, chunk, colorReset)
}

func (o *ConsoleOutput) OnTranslationEnd() {
	fmt.Println() // 换行
}

func (o *ConsoleOutput) OnInfo(msg string) {
	fmt.Printf("%s[信息] %s%s\n", colorYellow, msg, colorReset)
}

func (o *ConsoleOutput) OnError(msg string) {
	fmt.Printf("%s[错误] %s%s\n", colorRed, msg, colorReset)
}

// Writer 辅助类型，收集翻译输出到字符串
type Writer struct {
	strings.Builder
}

func NewWriter() *Writer {
	return &Writer{}
}
