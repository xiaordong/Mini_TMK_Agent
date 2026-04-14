//go:build !nocgo

package audio

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// sileroModel 直接封装 ONNX Runtime 的 Silero VAD 模型
// 基于 plandem/silero-go 的 Model 实现，但不依赖该第三方库
type sileroModel struct {
	session        *ort.AdvancedSession
	inputTensor    *ort.Tensor[float32]
	stateTensor    *ort.Tensor[float32]
	srTensor       *ort.Tensor[int64]
	outputTensor   *ort.Tensor[float32]
	newStateTensor *ort.Tensor[float32]
	context        []float32 // 上一帧的尾部采样（contextSize 个）
	windowSize     int       // 512 for 16kHz
	contextSize    int       // 64 for 16kHz
}

func (s *sileroModel) predict(chunk []float32) (float32, error) {
	if len(chunk) > s.windowSize {
		chunk = chunk[:s.windowSize]
	}

	// 拼接 context + chunk → input tensor
	data := s.inputTensor.GetData()
	for i := range data {
		data[i] = 0
	}
	copy(data, s.context)
	copy(data[len(s.context):], chunk)

	if err := s.session.Run(); err != nil {
		return 0, err
	}

	prob := s.outputTensor.GetData()[0]

	// 更新状态：newState → state
	copy(s.stateTensor.GetData(), s.newStateTensor.GetData())

	// 保存尾部采样作为下次 context
	copy(s.context, data[len(data)-len(s.context):])

	return prob, nil
}

func (s *sileroModel) destroy() {
	s.session.Destroy()
	s.inputTensor.Destroy()
	s.stateTensor.Destroy()
	s.srTensor.Destroy()
	s.outputTensor.Destroy()
	s.newStateTensor.Destroy()
}

// 全局 ONNX Runtime 初始化（只执行一次）
var (
	onnxOnce sync.Once
	onnxOK   bool
)

func initONNXRuntime() bool {
	onnxOnce.Do(func() {
		libName := "onnxruntime.dll"
		if runtime.GOOS == "linux" {
			libName = "libonnxruntime.so"
		} else if runtime.GOOS == "darwin" {
			libName = "libonnxruntime.dylib"
		}

		// 搜索 ONNX Runtime 共享库
		paths := []string{libName}
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			paths = append(paths,
				filepath.Join(dir, libName),
				filepath.Join(dir, "lib", libName),
			)
		}

		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				ort.SetSharedLibraryPath(p)
				if err := ort.InitializeEnvironment(); err == nil {
					onnxOK = true
					return
				}
			}
		}
	})
	return onnxOK
}

// findSileroModel 搜索 Silero ONNX 模型文件
func findSileroModel() string {
	candidates := []string{
		"silero_vad.onnx",
		"models/silero_vad.onnx",
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "silero_vad.onnx"),
			filepath.Join(dir, "models", "silero_vad.onnx"),
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// newSileroModel 创建 Silero VAD ONNX 模型实例
func newSileroModel(modelPath string) (*sileroModel, error) {
	if !ort.IsInitialized() {
		return nil, errONNXNotReady
	}

	const (
		sampleRate  = 16000
		windowSize  = 512
		contextSize = 64
		effWindow   = windowSize + contextSize // 576
	)

	// 创建 session options
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	defer opts.Destroy()
	opts.SetIntraOpNumThreads(1)
	opts.SetInterOpNumThreads(1)
	opts.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll)

	// Input tensor: [1, 576]
	inputTensor, err := ort.NewEmptyTensor[float32](ort.NewShape(1, effWindow))
	if err != nil {
		return nil, err
	}

	// State tensor: [2, 1, 128]
	stateTensor, err := ort.NewEmptyTensor[float32](ort.NewShape(2, 1, 128))
	if err != nil {
		inputTensor.Destroy()
		return nil, err
	}

	// Sample rate tensor: [1]
	srTensor, err := ort.NewTensor[int64](ort.NewShape(1), []int64{sampleRate})
	if err != nil {
		inputTensor.Destroy()
		stateTensor.Destroy()
		return nil, err
	}

	// Output tensor: [1, 1]
	outputTensor, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 1))
	if err != nil {
		inputTensor.Destroy()
		stateTensor.Destroy()
		srTensor.Destroy()
		return nil, err
	}

	// New state tensor: [2, 1, 128]
	newStateTensor, err := ort.NewEmptyTensor[float32](ort.NewShape(2, 1, 128))
	if err != nil {
		inputTensor.Destroy()
		stateTensor.Destroy()
		srTensor.Destroy()
		outputTensor.Destroy()
		return nil, err
	}

	session, err := ort.NewAdvancedSession(
		modelPath,
		[]string{"input", "state", "sr"},
		[]string{"output", "stateN"},
		[]ort.Value{inputTensor, stateTensor, srTensor},
		[]ort.Value{outputTensor, newStateTensor},
		opts,
	)
	if err != nil {
		inputTensor.Destroy()
		stateTensor.Destroy()
		srTensor.Destroy()
		outputTensor.Destroy()
		newStateTensor.Destroy()
		return nil, err
	}

	return &sileroModel{
		session:        session,
		inputTensor:    inputTensor,
		stateTensor:    stateTensor,
		srTensor:       srTensor,
		outputTensor:   outputTensor,
		newStateTensor: newStateTensor,
		context:        make([]float32, contextSize),
		windowSize:     windowSize,
		contextSize:    contextSize,
	}, nil
}

// tryInitSilero 尝试初始化 Silero VAD 模型
// 返回 nil 表示不可用，调用方应使用能量检测兜底
func tryInitSilero() vadModel {
	if !initONNXRuntime() {
		log.Println("[VAD] ONNX Runtime 不可用，使用能量检测兜底")
		return nil
	}

	modelPath := findSileroModel()
	if modelPath == "" {
		log.Println("[VAD] 未找到 silero_vad.onnx，使用能量检测兜底")
		return nil
	}

	model, err := newSileroModel(modelPath)
	if err != nil {
		log.Printf("[VAD] Silero 模型加载失败: %v，使用能量检测兜底", err)
		return nil
	}

	log.Printf("[VAD] Silero VAD 已启用（模型: %s）", modelPath)
	return model
}
