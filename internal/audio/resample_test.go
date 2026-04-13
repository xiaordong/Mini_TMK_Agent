package audio

import (
	"math"
	"testing"
)

func TestResample16_IdenticalRate(t *testing.T) {
	// 相同采样率应原样返回
	data := make([]byte, 100)
	result := Resample16(data, 16000)
	if len(result) != len(data) {
		t.Errorf("相同采样率应返回原始数据")
	}
}

func TestResample16_Downsample(t *testing.T) {
	// 48kHz -> 16kHz，3:1 下采样
	// 生成 480 个样本 (960 bytes) 的 48kHz 正弦波
	numSamples := 480
	samples := make([]byte, numSamples*2)
	for i := 0; i < numSamples; i++ {
		// 简单的正弦波
		v := int16(math.Sin(float64(i)/480*2*math.Pi) * 10000)
		samples[i*2] = byte(v)
		samples[i*2+1] = byte(v >> 8)
	}

	result := Resample16(samples, 48000)

	// 期望约 160 个样本 (320 bytes)
	expectedLen := 320
	if len(result) != expectedLen {
		t.Errorf("重采样后长度 = %d, 期望约 %d", len(result), expectedLen)
	}
}

func TestResample16_EmptyData(t *testing.T) {
	result := Resample16([]byte{}, 48000)
	if len(result) != 0 {
		t.Errorf("空数据应返回空结果")
	}

	// 少于 4 字节应原样返回
	result = Resample16([]byte{0x01, 0x00}, 48000)
	if len(result) != 2 {
		t.Errorf("短数据应原样返回")
	}
}

func TestMonoFromStereo(t *testing.T) {
	// 构造立体声数据: 左声道 = 100, 右声道 = 200
	stereo := make([]byte, 4)
	// left = 100
	stereo[0] = 100
	stereo[1] = 0
	// right = 200
	stereo[2] = 200
	stereo[3] = 0

	mono := MonoFromStereo(stereo)
	if len(mono) != 2 {
		t.Fatalf("单声道长度 = %d, 期望 2", len(mono))
	}

	val := int16(mono[0]) | int16(mono[1])<<8
	// (100 + 200) / 2 = 150
	if val != 150 {
		t.Errorf("均值 = %d, 期望 150", val)
	}
}

func TestComputeRMS(t *testing.T) {
	// 全零数据 RMS 应为 0
	zeroData := make([]byte, 100)
	rms := ComputeRMS(zeroData)
	if rms != 0 {
		t.Errorf("零数据 RMS = %f, 期望 0", rms)
	}

	// 空数据
	rms = ComputeRMS([]byte{})
	if rms != 0 {
		t.Errorf("空数据 RMS = %f, 期望 0", rms)
	}

	// 已知值: 2 个样本 [1000, -1000]
	data := make([]byte, 4)
	data[0] = 0xE8 // 1000 = 0x03E8
	data[1] = 0x03
	data[2] = 0x18 // -1000 = 0xFC18 (补码)
	data[3] = 0xFC

	rms = ComputeRMS(data)
	expected := 1000.0
	if math.Abs(rms-expected) > 1.0 {
		t.Errorf("RMS = %f, 期望约 %f", rms, expected)
	}
}
