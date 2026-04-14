//go:build nocgo

package audio

// tryInitSilero nocgo 模式下不可用，返回 nil 以使用能量检测兜底
func tryInitSilero() vadModel {
	return nil
}
