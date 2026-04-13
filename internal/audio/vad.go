package audio

import (
	"math"
)

// VADState VAD 状态
type VADState int

const (
	StateSilence VADState = iota
	StateSpeaking
)

// VAD 能量检测语音活动检测器
type VAD struct {
	state       VADState
	threshold   float64 // 能量阈值
	silenceMs   int     // 连续静音判定时间（毫秒）
	frameMs     int     // 帧大小（毫秒）
	silenceFrames int   // 连续静音帧计数
	speechBuf   []byte  // 累积的语音数据
	calibrating bool    // 是否在校准中
	calFrames   int     // 校准帧计数
	calEnergy   []float64 // 校准期间的能量值
}

// NewVAD 创建 VAD 检测器
// frameMs: 帧大小（毫秒），silenceMs: 连续静音判定时间（毫秒）
func NewVAD(frameMs, silenceMs int) *VAD {
	return &VAD{
		state:       StateSilence,
		silenceMs:   silenceMs,
		frameMs:     frameMs,
		calibrating: true,
		calFrames:   10, // 用前 10 帧校准底噪
		calEnergy:   make([]float64, 0, 10),
	}
}

// FrameSize 返回每帧的字节数 (16kHz 16bit mono)
func (v *VAD) FrameSize() int {
	return 16000 * 2 * v.frameMs / 1000 // = sampleRate * bytesPerSample * frameMs / 1000
}

// Process 处理一帧数据，返回是否检测到完整句子
// 返回值: (sentenceData []byte, isComplete bool)
func (v *VAD) Process(frame []byte) ([]byte, bool) {
	rms := ComputeRMS(frame)

	// 校准阶段
	if v.calibrating {
		v.calEnergy = append(v.calEnergy, rms)
		if len(v.calEnergy) >= v.calFrames {
			v.calibrate()
		}
		return nil, false
	}

	switch v.state {
	case StateSilence:
		if rms > v.threshold {
			v.state = StateSpeaking
			v.speechBuf = append(v.speechBuf, frame...)
			v.silenceFrames = 0
		}

	case StateSpeaking:
		v.speechBuf = append(v.speechBuf, frame...)
		if rms < v.threshold {
			v.silenceFrames++
		} else {
			v.silenceFrames = 0
		}

		// 连续静音超过阈值，判定句子结束
		silenceThreshold := v.silenceMs / v.frameMs
		if v.silenceFrames >= silenceThreshold {
			sentence := v.speechBuf
			v.speechBuf = nil
			v.state = StateSilence
			v.silenceFrames = 0
			return sentence, true
		}
	}

	return nil, false
}

// calibrate 根据校准帧计算能量阈值
func (v *VAD) calibrate() {
	if len(v.calEnergy) == 0 {
		v.threshold = 500 // 默认阈值
		v.calibrating = false
		return
	}

	// 取底噪平均值，乘以系数作为阈值
	var sum float64
	for _, e := range v.calEnergy {
		sum += e
	}
	avg := sum / float64(len(v.calEnergy))
	v.threshold = math.Max(avg*3, 300) // 至少 300，防止底噪极低时过于敏感
	v.calibrating = false
}

// SetThreshold 手动设置阈值（跳过校准）
func (v *VAD) SetThreshold(threshold float64) {
	v.threshold = threshold
	v.calibrating = false
}

// State 返回当前状态
func (v *VAD) State() VADState {
	return v.state
}

// Flush 返回当前缓冲区中未完成的语音数据（文件结束时调用）
func (v *VAD) Flush() []byte {
	if v.state == StateSpeaking && len(v.speechBuf) > 0 {
		data := v.speechBuf
		v.speechBuf = nil
		v.state = StateSilence
		return data
	}
	return nil
}
