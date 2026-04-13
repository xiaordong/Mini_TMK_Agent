package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Mini_TMK_Agent/internal/asr"
	"Mini_TMK_Agent/internal/audio"
	"Mini_TMK_Agent/internal/output"
	"Mini_TMK_Agent/internal/translate"
)

// writeLE32 写入小端 uint32
func writeLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func writeLE16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// createWAVFile 创建测试用 WAV 文件，模拟语音：高能量段 + 静音段交替
func createWAVFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "audio.wav")

	// 3 秒 PCM: 1s loud + 0.5s silence + 1s loud + 0.5s silence
	bytesPerSecond := 16000 * 2
	totalBytes := 3 * bytesPerSecond
	pcmData := make([]byte, totalBytes)

	// 高能量方波
	fillLoud := func(offset, length int) {
		for i := 0; i < length; i += 2 {
			if (i/2)%2 == 0 {
				pcmData[offset+i] = 0xFF
				pcmData[offset+i+1] = 0x7F
			} else {
				pcmData[offset+i] = 0x00
				pcmData[offset+i+1] = 0x80
			}
		}
	}

	fillLoud(0, bytesPerSecond)
	fillLoud(bytesPerSecond+bytesPerSecond/2, bytesPerSecond)

	// 构造正确的 WAV header
	header := make([]byte, 44)
	copy(header[0:4], []byte("RIFF"))
	writeLE32(header[4:8], uint32(36+len(pcmData)))
	copy(header[8:12], []byte("WAVE"))
	copy(header[12:16], []byte("fmt "))
	writeLE32(header[16:20], 16)
	writeLE16(header[20:22], 1)            // PCM
	writeLE16(header[22:24], 1)            // mono
	writeLE32(header[24:28], 16000)        // sample rate
	writeLE32(header[28:32], 32000)        // byte rate
	writeLE16(header[32:34], 2)            // block align
	writeLE16(header[34:36], 16)           // bits per sample
	copy(header[36:40], []byte("data"))
	writeLE32(header[40:44], uint32(len(pcmData)))

	wavData := append(header, pcmData...)
	if err := os.WriteFile(path, wavData, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFilePipeline_Run(t *testing.T) {
	asrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := asr.Result{Text: "你好世界", Language: "zh", Duration: 1.0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer asrServer.Close()

	transServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello World\"}}]}\n\n")
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer transServer.Close()

	filePath := createWAVFile(t)
	outPath := filepath.Join(t.TempDir(), "output.txt")

	pipe := NewFilePipeline(FilePipelineConfig{
		ASRProvider:   asr.NewProvider(asrServer.URL, "fake", "test"),
		TransProvider: translate.NewProvider(transServer.URL, "fake", "test"),
		Output:        output.NewConsoleOutput(false),
		SourceLang:    "zh",
		TargetLang:    "en",
	})

	err := pipe.Run(context.Background(), filePath, outPath)
	if err != nil {
		t.Fatalf("Run 出错: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读取输出文件失败: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "[原文] 你好世界") {
		t.Errorf("输出文件应包含原文: %q", content)
	}
	if !strings.Contains(content, "[译文] Hello World") {
		t.Errorf("输出文件应包含译文: %q", content)
	}
}

func TestFilePipeline_NoOutput(t *testing.T) {
	asrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"text":"","language":"","duration":0}`)
	}))
	defer asrServer.Close()

	filePath := createWAVFile(t)

	pipe := NewFilePipeline(FilePipelineConfig{
		ASRProvider:   asr.NewProvider(asrServer.URL, "fake", "test"),
		TransProvider: translate.NewProvider("http://localhost", "fake", "test"),
		Output:        output.NewConsoleOutput(false),
		SourceLang:    "auto",
		TargetLang:    "zh",
	})

	err := pipe.Run(context.Background(), filePath, "")
	if err != nil {
		t.Fatalf("Run 出错: %v", err)
	}
}

func TestSegmentByVAD(t *testing.T) {
	// 构造 PCM 数据：高能量 + 静音 + 高能量 + 静音
	bytesPerSecond := 16000 * 2
	totalBytes := 4 * bytesPerSecond // 4 秒
	pcmData := make([]byte, totalBytes)

	// 0-1s 高能量
	for i := 0; i < bytesPerSecond; i += 2 {
		if (i/2)%2 == 0 {
			pcmData[i] = 0xFF
			pcmData[i+1] = 0x7F
		} else {
			pcmData[i] = 0x00
			pcmData[i+1] = 0x80
		}
	}
	// 1-2s 静音 (已经是零)
	// 2-3s 高能量
	for i := 2 * bytesPerSecond; i < 3*bytesPerSecond; i += 2 {
		if ((i-2*bytesPerSecond)/2)%2 == 0 {
			pcmData[i] = 0xFF
			pcmData[i+1] = 0x7F
		} else {
			pcmData[i] = 0x00
			pcmData[i+1] = 0x80
		}
	}
	// 3-4s 静音

	pipe := &FilePipeline{}
	segments := pipe.segmentByVAD(pcmData)

	if len(segments) < 2 {
		t.Errorf("期望至少 2 个语音段，实际 %d 个", len(segments))
	}

	// 每段应该对应 1 秒高能量 + VAD 累积的静音尾部
	for i, seg := range segments {
		if len(seg) < bytesPerSecond/2 {
			t.Errorf("第 %d 段太短: %d bytes", i, len(seg))
		}
		if len(seg) > 2*bytesPerSecond {
			t.Errorf("第 %d 段太长: %d bytes", i, len(seg))
		}
	}
}

// TestRealAudioFile 用 Data 目录的真实音频测试 VAD 分段（如果文件存在）
func TestRealAudioFile(t *testing.T) {
	wavPath := filepath.Join("..", "..", "Data", "XiSay.wav")
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Skip("跳过：Data/XiSay.wav 不存在")
	}

	reader := audio.NewFileReader()
	pcmData, info, err := reader.Read(wavPath)
	if err != nil {
		t.Fatalf("读取 WAV 失败: %v", err)
	}

	t.Logf("音频时长: %.1f秒, 数据量: %d bytes", info.Duration, len(pcmData))

	pipe := &FilePipeline{}
	segments := pipe.segmentByVAD(pcmData)

	t.Logf("VAD 检测到 %d 个语音段", len(segments))
	if len(segments) == 0 {
		t.Error("应该检测到至少一个语音段")
	}

	totalDur := 0.0
	for i, seg := range segments {
		dur := float64(len(seg)/2) / 16000.0
		totalDur += dur
		t.Logf("  段 %d: %.1f秒 (%d bytes)", i+1, dur, len(seg))
	}
	t.Logf("总语音时长: %.1f秒 (原始 %.1f秒)", totalDur, info.Duration)
}

// mockCapturer 用于测试的 mock 采集器
type mockCapturer struct {
	data [][]byte
}

func (m *mockCapturer) Start(onData func([]byte)) error {
	for _, d := range m.data {
		onData(d)
	}
	return nil
}

func (m *mockCapturer) Stop() error { return nil }

func TestStreamPipeline_ASRLoop(t *testing.T) {
	asrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := asr.Result{Text: "测试文本", Language: "zh"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer asrServer.Close()

	speechChan := make(chan []byte, 1)
	textChan := make(chan *asr.Result, 1)

	speechChan <- make([]byte, 6400)
	close(speechChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := asr.NewProvider(asrServer.URL, "fake", "test")
	out := output.NewConsoleOutput(false)

	pipe := &StreamPipeline{
		asrProvider: provider,
		out:         out,
		sourceLang:  "zh",
	}

	go pipe.asrLoop(ctx, speechChan, textChan)

	result := <-textChan
	if result.Text != "测试文本" {
		t.Errorf("Text = %q, 期望 %q", result.Text, "测试文本")
	}
}
