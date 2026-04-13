package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AudioInfo 音频文件元信息
type AudioInfo struct {
	SampleRate int
	Channels   int
	Duration   float64 // 秒
}

// Reader 音频文件读取接口
type Reader interface {
	Read(filePath string) ([]byte, *AudioInfo, error)
}

// FileReader 默认文件读取实现
type FileReader struct{}

// NewFileReader 创建文件读取器
func NewFileReader() *FileReader {
	return &FileReader{}
}

func (r *FileReader) Read(filePath string) ([]byte, *AudioInfo, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".wav":
		return r.readWAV(filePath)
	case ".mp3":
		return r.readMP3(filePath)
	case ".pcm":
		return r.readPCM(filePath)
	default:
		return nil, nil, fmt.Errorf("不支持的音频格式: %s (支持 wav/mp3/pcm)", ext)
	}
}

// readPCM 直接读取原始 PCM 文件，假设 16kHz 16bit mono
func (r *FileReader) readPCM(filePath string) ([]byte, *AudioInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 PCM 文件失败: %w", err)
	}

	info := &AudioInfo{
		SampleRate: 16000,
		Channels:   1,
		Duration:   float64(len(data)/2) / 16000.0,
	}

	return data, info, nil
}

// readWAV 读取 WAV 文件并转为 16kHz mono PCM
func (r *FileReader) readWAV(filePath string) ([]byte, *AudioInfo, error) {
	data, info, err := readWAVFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	// 转为 mono
	if info.Channels == 2 {
		data = MonoFromStereo(data)
		info.Channels = 1
	}

	// 重采样到 16kHz
	if info.SampleRate != 16000 {
		data = Resample16(data, info.SampleRate)
		info.SampleRate = 16000
		info.Duration = float64(len(data)/2) / 16000.0
	}

	return data, info, nil
}

// readMP3 使用 go-mp3 解码为 PCM 再处理
func (r *FileReader) readMP3(filePath string) ([]byte, *AudioInfo, error) {
	return decodeMP3(filePath)
}
