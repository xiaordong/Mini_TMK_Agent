package audio

import (
	"fmt"
	"os"

	"github.com/hajimehoshi/go-mp3"
)

// decodeMP3 解码 MP3 文件为 16kHz mono 16bit PCM
func decodeMP3(filePath string) ([]byte, *AudioInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 MP3 文件失败: %w", err)
	}
	defer f.Close()

	decoder, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, nil, fmt.Errorf("MP3 解码失败: %w", err)
	}

	// go-mp3 输出 16bit stereo PCM
	sampleRate := decoder.SampleRate()
	var pcmData []byte
	buf := make([]byte, 4096)

	for {
		n, err := decoder.Read(buf)
		if n > 0 {
			pcmData = append(pcmData, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	// go-mp3 输出是 stereo，转 mono
	pcmData = MonoFromStereo(pcmData)

	// 重采样到 16kHz
	if sampleRate != 16000 {
		pcmData = Resample16(pcmData, sampleRate)
	}

	info := &AudioInfo{
		SampleRate: 16000,
		Channels:   1,
		Duration:   float64(len(pcmData)/2) / 16000.0,
	}

	return pcmData, info, nil
}
