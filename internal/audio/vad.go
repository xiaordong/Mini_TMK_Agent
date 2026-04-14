package audio

import (
	"errors"
	"fmt"
	"math"
)

// errONNXNotReady ONNX Runtime 未初始化
var errONNXNotReady = errors.New("onnxruntime 未初始化")

// VADState VAD 状态
type VADState int

const (
	StateSilence VADState = iota
	StateSpeaking
)

// Silero VAD 参数
const (
	speechThreshold float32 = 0.5  // 语音概率阈值
	noiseThreshold  float32 = 0.35 // 低于此值判定为非语音（滞回机制）
	sileroWindow    int     = 512  // 16kHz 下的窗口大小（32ms）
)

// vadModel Silero VAD 模型接口（由 vad_silero.go 或 vad_nocgo_stub.go 实现）
type vadModel interface {
	predict(chunk []float32) (float32, error)
	destroy()
}

// VAD 语音活动检测器（Silero VAD 优先，能量检测兜底）
type VAD struct {
	state         VADState
	silenceMs     int     // 连续静音判定时间（毫秒）
	frameMs       int     // 帧大小（毫秒）
	silenceFrames int     // 连续静音帧/窗口计数
	speechBuf     []byte  // 累积的语音数据（int16 PCM）

	// Silero VAD（nil 时使用能量检测兜底）
	silero    vadModel
	windowBuf []float32 // float32 PCM 窗口缓冲

	// 能量检测兜底参数
	threshold   float64
	calibrating bool
	calFrames   int
	calEnergy   []float64
}

// NewVAD 创建 VAD 检测器
// frameMs: 帧大小（毫秒），silenceMs: 连续静音判定时间（毫秒）
func NewVAD(frameMs, silenceMs int) *VAD {
	v := &VAD{
		state:       StateSilence,
		silenceMs:   silenceMs,
		frameMs:     frameMs,
		// 能量检测兜底默认值
		calibrating: true,
		calFrames:   10,
		calEnergy:   make([]float64, 0, 10),
		threshold:   300,
	}

	// 尝试初始化 Silero VAD
	if m := tryInitSilero(); m != nil {
		v.silero = m
		v.calibrating = false
	}

	return v
}

// FrameSize 返回每帧的字节数 (16kHz 16bit mono)
func (v *VAD) FrameSize() int {
	return 16000 * 2 * v.frameMs / 1000
}

// Process 处理一帧数据，返回是否检测到完整句子
// 返回值: (sentenceData []byte, isComplete bool)
func (v *VAD) Process(frame []byte) ([]byte, bool) {
	if v.silero != nil {
		return v.processSilero(frame)
	}
	return v.processEnergy(frame)
}

// processSilero 使用 Silero VAD 处理音频
// 将 int16 PCM 转 float32，按 512 采样窗口送入 Silero 模型推理
func (v *VAD) processSilero(frame []byte) ([]byte, bool) {
	// int16 PCM → float32（归一化到 [-1, 1]）
	samples := bytesToSamples(frame)
	floats := make([]float32, len(samples))
	for i, s := range samples {
		floats[i] = float32(s) / 32768.0
	}
	v.windowBuf = append(v.windowBuf, floats...)

	// 逐窗口推理（每窗口 512 采样 ≈ 32ms）
	for len(v.windowBuf) >= sileroWindow {
		window := make([]float32, sileroWindow)
		copy(window, v.windowBuf[:sileroWindow])
		v.windowBuf = v.windowBuf[sileroWindow:]

		prob, err := v.silero.predict(window)
		if err != nil {
			continue
		}

		switch v.state {
		case StateSilence:
			if prob >= speechThreshold {
				v.state = StateSpeaking
				v.silenceFrames = 0
			}
		case StateSpeaking:
			if prob < noiseThreshold {
				v.silenceFrames++
			} else {
				v.silenceFrames = 0
			}
		}
	}

	// 累积原始 int16 PCM
	if v.state == StateSpeaking {
		v.speechBuf = append(v.speechBuf, frame...)
	}

	// 连续低概率窗口达到静音阈值 → 句子完成
	silenceThreshold := v.silenceMs / 32 // 每窗口 32ms
	if v.state == StateSpeaking && v.silenceFrames >= silenceThreshold {
		sentence := v.speechBuf
		v.speechBuf = nil
		v.state = StateSilence
		v.silenceFrames = 0
		return sentence, true
	}

	return nil, false
}

// processEnergy 使用能量检测处理（兜底方案）
func (v *VAD) processEnergy(frame []byte) ([]byte, bool) {
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
		v.threshold = 500
		v.calibrating = false
		return
	}

	var sum float64
	for _, e := range v.calEnergy {
		sum += e
	}
	avg := sum / float64(len(v.calEnergy))
	v.threshold = math.Min(math.Max(avg*1.5, 150), 600)
	v.calibrating = false
}

// SetThreshold 手动设置阈值（仅能量检测模式有效）
func (v *VAD) SetThreshold(threshold float64) {
	v.threshold = threshold
	v.calibrating = false
}

// State 返回当前状态
func (v *VAD) State() VADState {
	return v.state
}

// Threshold 返回当前阈值
func (v *VAD) Threshold() float64 {
	if v.silero != nil {
		return float64(speechThreshold)
	}
	return v.threshold
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

// Destroy 释放 VAD 资源（Silero 模型等）
func (v *VAD) Destroy() {
	if v.silero != nil {
		v.silero.destroy()
		v.silero = nil
	}
}

// VADMode 返回当前 VAD 模式描述
func (v *VAD) VADMode() string {
	if v.silero != nil {
		return fmt.Sprintf("Silero VAD (阈值=%.2f/%.2f)", speechThreshold, noiseThreshold)
	}
	return fmt.Sprintf("能量检测 (阈值=%.0f)", v.threshold)
}
