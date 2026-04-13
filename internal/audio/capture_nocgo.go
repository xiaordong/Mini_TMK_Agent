//go:build nocgo

package audio

import "fmt"

// Capturer 麦克风采集接口
type Capturer interface {
	Start(onData func([]byte)) error
	Stop() error
}

// MalgoCapturer CGO 不可用时的 stub 实现
type MalgoCapturer struct{}

func NewMalgoCapturer(sampleRate uint32) *MalgoCapturer {
	return &MalgoCapturer{}
}

func (c *MalgoCapturer) Start(onData func([]byte)) error {
	return fmt.Errorf("麦克风采集需要 CGO 支持，请安装 GCC 并重新编译")
}

func (c *MalgoCapturer) Stop() error {
	return nil
}
