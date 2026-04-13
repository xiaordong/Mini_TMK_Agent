package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureOutput 捕获 stdout 输出
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestConsoleOutput_OnTranslatedText(t *testing.T) {
	out := captureOutput(func() {
		o := NewConsoleOutput(false)
		o.OnTranslatedText("Hello ")
		o.OnTranslatedText("World")
		o.OnTranslationEnd()
	})

	// 去掉 ANSI 码后检查
	clean := strings.ReplaceAll(out, colorCyan, "")
	clean = strings.ReplaceAll(clean, colorReset, "")
	if !strings.Contains(clean, "Hello World") {
		t.Errorf("输出应包含 'Hello World': %q", clean)
	}
	if !strings.Contains(out, colorCyan) {
		t.Error("输出应包含青色 ANSI 码")
	}
}

func TestConsoleOutput_Verbose(t *testing.T) {
	out := captureOutput(func() {
		o := NewConsoleOutput(true)
		o.OnSourceText("你好")
	})

	if !strings.Contains(out, "[原文]") {
		t.Error("verbose 模式应显示原文")
	}
	if !strings.Contains(out, "你好") {
		t.Error("应包含原文内容")
	}
}

func TestConsoleOutput_NonVerbose(t *testing.T) {
	out := captureOutput(func() {
		o := NewConsoleOutput(false)
		o.OnSourceText("你好")
	})

	if strings.Contains(out, "[原文]") {
		t.Error("非 verbose 模式不应显示原文")
	}
}

func TestConsoleOutput_OnInfo(t *testing.T) {
	out := captureOutput(func() {
		o := NewConsoleOutput(false)
		o.OnInfo("开始录音")
	})

	if !strings.Contains(out, "[信息]") {
		t.Error("应包含 [信息]")
	}
}

func TestConsoleOutput_OnError(t *testing.T) {
	out := captureOutput(func() {
		o := NewConsoleOutput(false)
		o.OnError("连接失败")
	})

	if !strings.Contains(out, "[错误]") {
		t.Error("应包含 [错误]")
	}
	if !strings.Contains(out, colorRed) {
		t.Error("应包含红色 ANSI 码")
	}
}
