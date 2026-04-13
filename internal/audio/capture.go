//go:build !nocgo

package audio

import (
	"fmt"

	"github.com/gen2brain/malgo"
)

// Capturer 麦克风采集接口
type Capturer interface {
	Start(onData func([]byte)) error
	Stop() error
}

// MalgoCapturer 基于 malgo 的麦克风采集实现
type MalgoCapturer struct {
	ctx        *malgo.AllocatedContext
	device     *malgo.Device
	sampleRate uint32
}

// NewMalgoCapturer 创建麦克风采集器
func NewMalgoCapturer(sampleRate uint32) *MalgoCapturer {
	return &MalgoCapturer{
		sampleRate: sampleRate,
	}
}

func (c *MalgoCapturer) Start(onData func([]byte)) error {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return fmt.Errorf("初始化音频上下文失败: %w", err)
	}
	c.ctx = ctx

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = c.sampleRate

	onRecv := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		data := make([]byte, len(pInputSamples))
		copy(data, pInputSamples)
		onData(data)
	}

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onRecv,
	})
	if err != nil {
		c.ctx.Free()
		c.ctx = nil
		return fmt.Errorf("初始化采集设备失败: %w", err)
	}
	c.device = device

	if err := device.Start(); err != nil {
		c.device.Uninit()
		c.device = nil
		c.ctx.Free()
		c.ctx = nil
		return fmt.Errorf("启动采集设备失败: %w", err)
	}

	return nil
}

func (c *MalgoCapturer) Stop() error {
	if c.device != nil {
		c.device.Stop()
		c.device.Uninit()
	}
	if c.ctx != nil {
		c.ctx.Free()
	}
	return nil
}
