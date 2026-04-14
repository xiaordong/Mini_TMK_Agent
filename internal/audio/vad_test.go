package audio

import (
	"testing"
)

func TestVAD_SilenceToSpeaking(t *testing.T) {
	vad := NewVAD(200, 1200)
	vad.SetThreshold(500) // 跳过校准

	// 静音帧
	frame := make([]byte, vad.FrameSize())
	sentence, complete := vad.Process(frame)
	if complete {
		t.Error("静音帧不应触发句子完成")
	}
	if sentence != nil {
		t.Error("静音帧不应返回数据")
	}
	if vad.State() != StateSilence {
		t.Error("应该处于静音状态")
	}

	// 高能量帧（模拟语音）
	loudFrame := make([]byte, vad.FrameSize())
	for i := 1; i < len(loudFrame); i += 2 {
		loudFrame[i] = 0x7F // 高能量
	}

	sentence, complete = vad.Process(loudFrame)
	if complete {
		t.Error("刚开始说话不应触发句子完成")
	}
	if vad.State() != StateSpeaking {
		t.Error("应该处于说话状态")
	}
}

func TestVAD_SpeakingToSilence(t *testing.T) {
	vad := NewVAD(200, 600) // 200ms 帧，600ms 静音 = 3 帧静音判定
	vad.SetThreshold(500)

	// 先进入说话状态
	loudFrame := make([]byte, vad.FrameSize())
	for i := 1; i < len(loudFrame); i += 2 {
		loudFrame[i] = 0x7F
	}
	vad.Process(loudFrame)

	// 3 帧静音应触发句子完成
	silenceFrame := make([]byte, vad.FrameSize())
	for i := 0; i < 2; i++ {
		vad.Process(silenceFrame)
	}

	sentence, complete := vad.Process(silenceFrame)
	if !complete {
		t.Error("连续静音应触发句子完成")
	}
	if len(sentence) == 0 {
		t.Error("句子数据不应为空")
	}
	if vad.State() != StateSilence {
		t.Error("完成后应回到静音状态")
	}
}

func TestVAD_Calibration(t *testing.T) {
	vad := NewVAD(200, 1200)

	// 前 10 帧用于校准
	silenceFrame := make([]byte, vad.FrameSize())
	for i := 0; i < 10; i++ {
		vad.Process(silenceFrame)
	}

	if vad.calibrating {
		t.Error("校准应该已完成")
	}
	if vad.threshold < 150 {
		t.Errorf("阈值 = %f, 不应低于 150", vad.threshold)
	}
}

func TestVAD_FrameSize(t *testing.T) {
	vad := NewVAD(200, 1200)
	// 16kHz * 2 bytes * 200ms / 1000 = 6400
	expected := 6400
	if vad.FrameSize() != expected {
		t.Errorf("FrameSize = %d, 期望 %d", vad.FrameSize(), expected)
	}
}
