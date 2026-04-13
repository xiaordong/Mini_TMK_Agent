package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// readWAVFile 读取 WAV 文件，正确处理 LIST 等额外 chunk
func readWAVFile(filePath string) ([]byte, *AudioInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 WAV 文件失败: %w", err)
	}
	defer f.Close()

	// 读取 RIFF header (12 bytes)
	riffHeader := make([]byte, 12)
	if _, err := io.ReadFull(f, riffHeader); err != nil {
		return nil, nil, fmt.Errorf("读取 RIFF header 失败: %w", err)
	}
	if string(riffHeader[0:4]) != "RIFF" || string(riffHeader[8:12]) != "WAVE" {
		return nil, nil, fmt.Errorf("不是有效的 WAV 文件")
	}

	var (
		channels     int
		sampleRate   int
		bitsPerSample int
		audioFormat  uint16
		fmtFound     bool
		pcmData      []byte
	)

	// 遍历子块
	for {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(f, chunkHeader); err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, fmt.Errorf("读取 chunk header 失败: %w", err)
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		switch chunkID {
		case "fmt ":
			fmtData := make([]byte, chunkSize)
			if _, err := io.ReadFull(f, fmtData); err != nil {
				return nil, nil, fmt.Errorf("读取 fmt chunk 失败: %w", err)
			}
			if len(fmtData) < 16 {
				return nil, nil, fmt.Errorf("fmt chunk 太小")
			}
			audioFormat = binary.LittleEndian.Uint16(fmtData[0:2])
			if audioFormat != 1 {
				return nil, nil, fmt.Errorf("不支持的音频格式: %d (仅支持 PCM)", audioFormat)
			}
			channels = int(binary.LittleEndian.Uint16(fmtData[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(fmtData[4:8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(fmtData[14:16]))
			if bitsPerSample != 16 {
				return nil, nil, fmt.Errorf("不支持 %d 位采样深度 (仅支持 16bit)", bitsPerSample)
			}
			fmtFound = true

		case "data":
			pcmData = make([]byte, chunkSize)
			if _, err := io.ReadFull(f, pcmData); err != nil {
				return nil, nil, fmt.Errorf("读取 PCM 数据失败: %w", err)
			}
			// 找到 data chunk 后就可以停了
			goto done

		default:
			// 跳过其他 chunk（LIST、INFO 等）
			if _, err := f.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return nil, nil, fmt.Errorf("跳过 chunk %s 失败: %w", chunkID, err)
			}
		}
	}

done:
	if !fmtFound {
		return nil, nil, fmt.Errorf("WAV 文件缺少 fmt chunk")
	}
	if pcmData == nil {
		return nil, nil, fmt.Errorf("WAV 文件缺少 data chunk")
	}

	numSamples := len(pcmData) / (bitsPerSample / 8) / channels
	duration := float64(numSamples) / float64(sampleRate)

	info := &AudioInfo{
		SampleRate: sampleRate,
		Channels:   channels,
		Duration:   duration,
	}

	return pcmData, info, nil
}
