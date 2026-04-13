package output

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockProvider 用于测试的 TTS Provider mock
type mockProvider struct {
	synthesized []string // 记录收到的文本
	audioData   []byte   // 返回的音频数据
	err         error    // 模拟错误
}

func (m *mockProvider) Synthesize(ctx context.Context, text, lang string) ([]byte, error) {
	m.synthesized = append(m.synthesized, text)
	if m.err != nil {
		return nil, m.err
	}
	return m.audioData, nil
}

func TestTTSOutput_OnTranslatedText_Buffers(t *testing.T) {
	mock := &mockProvider{audioData: []byte("audio")}
	out := NewTTSOutput(NewConsoleOutput(false), TTSOutputConfig{
		Provider:   mock,
		TargetLang: "en",
	})

	// 模拟流式翻译
	out.OnTranslatedText("Hello")
	out.OnTranslatedText(" World")
	out.OnTranslationEnd()

	if len(mock.synthesized) != 1 {
		t.Fatalf("Synthesize 调用次数 = %d, 期望 1", len(mock.synthesized))
	}
	if mock.synthesized[0] != "Hello World" {
		t.Errorf("合成文本 = %q, 期望 %q", mock.synthesized[0], "Hello World")
	}

	// 验证 segments 被收集
	out.mu.Lock()
	segs := len(out.segments)
	out.mu.Unlock()
	if segs != 1 {
		t.Errorf("segments 数量 = %d, 期望 1", segs)
	}
}

func TestTTSOutput_OnTranslationEnd_SkipsEmpty(t *testing.T) {
	mock := &mockProvider{audioData: []byte("audio")}
	out := NewTTSOutput(NewConsoleOutput(false), TTSOutputConfig{
		Provider:   mock,
		TargetLang: "en",
	})

	// 没有写入任何文本就调用 OnTranslationEnd
	out.OnTranslationEnd()

	if len(mock.synthesized) != 0 {
		t.Errorf("空文本不应调用 Synthesize")
	}
}

func TestTTSOutput_Flush_FileMode(t *testing.T) {
	mock := &mockProvider{audioData: []byte("fake-audio-data")}
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.txt")

	out := NewTTSOutput(NewConsoleOutput(false), TTSOutputConfig{
		Provider:   mock,
		TargetLang: "en",
		OutputPath: outputPath,
	})

	out.OnTranslatedText("Hello")
	out.OnTranslationEnd()

	err := out.Flush()
	if err != nil {
		t.Fatalf("Flush 出错: %v", err)
	}

	// 验证 MP3 文件已生成
	mp3Path := filepath.Join(tmpDir, "output.mp3")
	data, err := os.ReadFile(mp3Path)
	if err != nil {
		t.Fatalf("读取 MP3 文件失败: %v", err)
	}
	if string(data) != "fake-audio-data" {
		t.Errorf("MP3 文件内容不匹配")
	}
}

func TestTTSOutput_Flush_StreamMode(t *testing.T) {
	mock := &mockProvider{audioData: []byte("seg-audio")}

	out := NewTTSOutput(NewConsoleOutput(false), TTSOutputConfig{
		Provider:   mock,
		TargetLang: "zh",
	})

	out.OnTranslatedText("第一段")
	out.OnTranslationEnd()
	out.OnTranslatedText("第二段")
	out.OnTranslationEnd()

	// 切换到临时目录保存流模式的分段文件
	oldDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	err := out.Flush()
	if err != nil {
		t.Fatalf("Flush 出错: %v", err)
	}

	// 验证两个分段文件已生成
	for _, name := range []string{"segment_001.mp3", "segment_002.mp3"} {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			t.Errorf("文件 %s 未生成", name)
		}
	}
}

func TestTTSOutput_Flush_NoSegments(t *testing.T) {
	mock := &mockProvider{audioData: []byte("audio")}
	out := NewTTSOutput(NewConsoleOutput(false), TTSOutputConfig{
		Provider:   mock,
		TargetLang: "en",
	})

	// 没有 segments，Flush 应该直接返回 nil
	err := out.Flush()
	if err != nil {
		t.Errorf("没有 segments 时 Flush 不应报错: %v", err)
	}
}

func TestTTSOutput_MultipleSegments_Merged(t *testing.T) {
	callCount := 0
	mock := &mockProvider{
		audioData: []byte("audio"),
	}
	out := NewTTSOutput(NewConsoleOutput(false), TTSOutputConfig{
		Provider:   mock,
		TargetLang: "en",
	})

	// 模拟多段翻译
	out.OnTranslatedText("段1")
	out.OnTranslationEnd()
	callCount++

	out.OnTranslatedText("段2")
	out.OnTranslationEnd()
	callCount++

	if len(mock.synthesized) != callCount {
		t.Errorf("Synthesize 调用次数 = %d, 期望 %d", len(mock.synthesized), callCount)
	}

	out.mu.Lock()
	segs := len(out.segments)
	out.mu.Unlock()
	if segs != 2 {
		t.Errorf("segments 数量 = %d, 期望 2", segs)
	}

	// 验证两段文本
	if mock.synthesized[0] != "段1" || mock.synthesized[1] != "段2" {
		t.Errorf("合成文本顺序不正确: %v", mock.synthesized)
	}

	// 验证合并
	var merged []byte
	for _, s := range out.segments {
		merged = append(merged, s...)
	}
	expected := strings.Repeat("audio", 2)
	if string(merged) != expected {
		t.Errorf("合并音频不匹配")
	}
}
