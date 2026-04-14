.PHONY: build build-full build-lite run test clean

BINARY=mini-tmk-agent

# 自动检测 GCC
HAS_GCC := $(shell gcc --version 2>/dev/null)

ifeq ($(HAS_GCC),)
BUILD_TAGS = -tags nocgo
else
BUILD_TAGS =
endif

# 自动选择最佳模式
build:
	go build $(BUILD_TAGS) -ldflags "-s -w" -o $(BINARY) ./cmd/mini-tmk-agent

# 强制 CGO（麦克风 + Silero VAD）
build-full:
	go build -ldflags "-s -w" -o $(BINARY) ./cmd/mini-tmk-agent

# 纯 Go（仅文件转录 + Web）
build-lite:
	go build -tags nocgo -ldflags "-s -w" -o $(BINARY) ./cmd/mini-tmk-agent

run: build
	./$(BINARY)

test:
	go test ./...

test-integration:
	go test -tags integration ./...

clean:
	rm -f $(BINARY)
