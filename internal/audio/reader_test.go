package audio

import (
	"os"
	"path/filepath"
	"testing"
)

// 创建一个简单的 WAV 测试文件
func createWAVFile(t *testing.T, sampleRate, channels int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "audio.wav")

	// 生成 1000 字节 PCM 数据
	pcmData := make([]byte, 1000)
	for i := range pcmData {
		pcmData[i] = byte(i % 256)
	}

	// 构造 WAV header
	header := make([]byte, 44)
	copy(header[0:4], []byte("RIFF"))
	writeLE32(header[4:8], uint32(36+len(pcmData)))
	copy(header[8:12], []byte("WAVE"))
	copy(header[12:16], []byte("fmt "))
	writeLE32(header[16:20], 16)
	writeLE16(header[20:22], 1) // PCM
	writeLE16(header[22:24], uint16(channels))
	writeLE32(header[24:28], uint32(sampleRate))
	bytesPerSample := uint16(2)
	writeLE32(header[28:32], uint32(sampleRate)*uint32(channels)*uint32(bytesPerSample))
	writeLE16(header[32:34], uint16(channels)*bytesPerSample)
	writeLE16(header[34:36], bytesPerSample*8) // bits
	copy(header[36:40], []byte("data"))
	writeLE32(header[40:44], uint32(len(pcmData)))

	wavData := append(header, pcmData...)
	if err := os.WriteFile(path, wavData, 0644); err != nil {
		t.Fatal(err)
	}

	return path
}

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

func TestFileReader_ReadWAV(t *testing.T) {
	path := createWAVFile(t, 16000, 1)
	reader := NewFileReader()

	data, info, err := reader.Read(path)
	if err != nil {
		t.Fatalf("Read 出错: %v", err)
	}
	if info.SampleRate != 16000 {
		t.Errorf("SampleRate = %d, 期望 16000", info.SampleRate)
	}
	if info.Channels != 1 {
		t.Errorf("Channels = %d, 期望 1", info.Channels)
	}
	if len(data) != 1000 {
		t.Errorf("数据长度 = %d, 期望 1000", len(data))
	}
}

func TestFileReader_ReadWAV_Resample(t *testing.T) {
	// 48kHz stereo -> 16kHz mono
	path := createWAVFile(t, 48000, 2)
	reader := NewFileReader()

	data, info, err := reader.Read(path)
	if err != nil {
		t.Fatalf("Read 出错: %v", err)
	}
	if info.SampleRate != 16000 {
		t.Errorf("SampleRate = %d, 期望 16000", info.SampleRate)
	}
	if info.Channels != 1 {
		t.Errorf("Channels = %d, 期望 1", info.Channels)
	}
	// 48kHz stereo 1000 bytes -> mono 500 bytes -> resample to 16kHz ~167 bytes
	if len(data) == 0 {
		t.Error("数据不应为空")
	}
}

func TestFileReader_ReadPCM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audio.pcm")
	pcmData := make([]byte, 32000) // 1 秒的 16kHz 16bit mono
	if err := os.WriteFile(path, pcmData, 0644); err != nil {
		t.Fatal(err)
	}

	reader := NewFileReader()
	data, info, err := reader.Read(path)
	if err != nil {
		t.Fatalf("Read 出错: %v", err)
	}
	if info.SampleRate != 16000 {
		t.Errorf("SampleRate = %d", info.SampleRate)
	}
	if len(data) != 32000 {
		t.Errorf("数据长度 = %d, 期望 32000", len(data))
	}
}

func TestFileReader_UnsupportedFormat(t *testing.T) {
	reader := NewFileReader()
	_, _, err := reader.Read("audio.flac")
	if err == nil {
		t.Error("不支持的格式应返回错误")
	}
}
