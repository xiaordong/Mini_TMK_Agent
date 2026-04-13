// Package audio 提供音频采集、读取、重采样和 VAD 功能
package audio

import "math"

// Resample16 将 16bit PCM 数据从 origRate 重采样到 16kHz
// 使用线性插值，输入为 16bit signed little-endian mono PCM
func Resample16(pcmData []byte, origRate int) []byte {
	if origRate == 16000 || len(pcmData) < 4 {
		return pcmData
	}

	targetRate := 16000
	numSamples := len(pcmData) / 2
	samples := bytesToSamples(pcmData)

	ratio := float64(origRate) / float64(targetRate)
	newLen := int(float64(numSamples) / ratio)
	if newLen < 1 {
		newLen = 1
	}

	result := make([]int16, newLen)
	for i := 0; i < newLen; i++ {
		srcPos := float64(i) * ratio
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)

		if srcIdx+1 < numSamples {
			// 线性插值
			result[i] = int16(float64(samples[srcIdx])*(1-frac) + float64(samples[srcIdx+1])*frac)
		} else if srcIdx < numSamples {
			result[i] = samples[srcIdx]
		}
	}

	return samplesToBytes(result)
}

// MonoFromStereo 将立体声 16bit PCM 转为单声道（取左右声道均值）
func MonoFromStereo(stereoData []byte) []byte {
	numFrames := len(stereoData) / 4 // 每帧 4 字节（2声道 × 2字节）
	mono := make([]byte, numFrames*2)

	for i := 0; i < numFrames; i++ {
		left := int16(stereoData[i*4]) | int16(stereoData[i*4+1])<<8
		right := int16(stereoData[i*4+2]) | int16(stereoData[i*4+3])<<8
		avg := int16((int32(left) + int32(right)) / 2)

		mono[i*2] = byte(avg)
		mono[i*2+1] = byte(avg >> 8)
	}

	return mono
}

// ComputeRMS 计算 16bit PCM 数据的 RMS 能量
func ComputeRMS(pcmData []byte) float64 {
	if len(pcmData) < 2 {
		return 0
	}
	samples := bytesToSamples(pcmData)
	var sum float64
	for _, s := range samples {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func bytesToSamples(data []byte) []int16 {
	n := len(data) / 2
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = int16(data[i*2]) | int16(data[i*2+1])<<8
	}
	return samples
}

func samplesToBytes(samples []int16) []byte {
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		data[i*2] = byte(s)
		data[i*2+1] = byte(s >> 8)
	}
	return data
}
