# Mini TMK Agent

同声传译 Agent — 麦克风实时翻译 & 音频文件转录 & Web UI，支持中/英/西/日四种语言互译。

## 3 分钟快速上手

> 前提：已安装 [Go 1.21+](https://go.dev/dl/)

```bash
# 1. 克隆并编译
git clone <repo-url> && cd Mini_TMK_Agent
make build

# 2. 配置（一次性）
./mini-tmk-agent config set provider siliconflow
./mini-tmk-agent config set api-key sk-your-key-here

# 3. 开始使用
./mini-tmk-agent transcript input.wav
```

### 免费获取 API Key

推荐使用 **SiliconFlow**，注册即送免费额度，中文识别效果好：

| 平台 | 注册地址 | 特点 |
|------|---------|------|
| **SiliconFlow** | [cloud.siliconflow.cn](https://cloud.siliconflow.cn/) | 中文好，免费额度充足 |
| **Groq** | [console.groq.com](https://console.groq.com/) | 速度极快，免费额度充足 |

---

## 命令参考

```
mini-tmk-agent stream <file> [output] [-s auto] [-t zh] [-v]   实时麦克风同传
mini-tmk-agent transcript <file> [output] [-s auto] [-t zh]     音频文件转录
mini-tmk-agent web [-p 8080]                                    Web UI
mini-tmk-agent config show                                      显示当前配置
mini-tmk-agent config set <key> <value>                         设置配置项
mini-tmk-agent config check                                     检查配置和运行环境
mini-tmk-agent version                                          版本信息
```

### 全局 Flags

| Flag | 短 | 默认 | 说明 |
|------|----|------|------|
| `--provider` | | 配置文件值 | 统一服务商 (groq/siliconflow/openai) |
| `-s, --source` | `-s` | `auto` | 源语言 |
| `-t, --target` | `-t` | `zh` | 目标语言 |
| `-v, --verbose` | `-v` | `false` | 显示原文 |

### `stream` — 实时麦克风同传

```bash
./mini-tmk-agent stream -s zh -t en -v
```

按 `Ctrl+C` 停止。需要 GCC 支持（`make build-full`）。

### `transcript` — 音频文件转录

```bash
# 基本用法
./mini-tmk-agent transcript input.wav

# 指定输出文件
./mini-tmk-agent transcript input.wav output.txt

# 指定语言和服务商
./mini-tmk-agent transcript speech.mp3 -s en -t zh --provider groq
```

支持 `.wav`、`.mp3`、`.pcm` 格式。

### `web` — Web UI

```bash
./mini-tmk-agent web          # 默认 8080 端口
./mini-tmk-agent web -p 3000  # 指定端口
```

浏览器打开 `http://localhost:8080`，支持：

- **文件转录**：拖拽上传音频文件，流式显示转录+翻译结果，带进度条
- **实时同传**：浏览器麦克风采集，WebSocket 实时传输 ASR+翻译结果
- **TTS 播放**：开启后翻译结果带播放按钮，一键收听译文语音
- **配置管理**：在线切换 Provider、语言对，设置 API Key

### `config` — 配置管理

```bash
# 设置统一服务商和 API Key
./mini-tmk-agent config set provider siliconflow
./mini-tmk-agent config set api-key sk-xxx

# 启用 TTS
./mini-tmk-agent config set tts true
./mini-tmk-agent config set tts-voice FunAudioLLM/CosyVoice2-0.5B:alex

# 查看当前配置（Key 脱敏显示）
./mini-tmk-agent config show

# 检查配置和运行环境
./mini-tmk-agent config check
```

可设置的 key：`provider`、`api-key`、`tts`、`tts-voice`

---

## TTS 语音合成（可选）

```bash
# 在配置中启用 TTS
./mini-tmk-agent config set tts true

# 转录 + TTS（生成 output.txt + output.mp3）
./mini-tmk-agent transcript input.wav output.txt
```

也可通过环境变量启用：

```env
TMK_TTS_ENABLED=true
```

---

## 配置

配置优先级（高 → 低）：

```
命令行 flags  >  环境变量  >  .env 文件  >  ~/.mini-tmk-agent.yaml  >  默认值
```

### 环境变量

| 变量名 | 说明 |
|--------|------|
| `TMK_API_KEY` | **统一 API Key**（三个服务共用） |
| `TMK_PROVIDER` | 统一服务商 (groq/siliconflow/openai) |
| `TMK_ASR_API_KEY` | ASR 专用 Key（优先级高于统一 Key） |
| `TMK_ASR_PROVIDER` | ASR 服务商 |
| `TMK_ASR_BASE_URL` | 自定义 ASR API 地址 |
| `TMK_ASR_MODEL` | 自定义 ASR 模型 |
| `TMK_TRANS_API_KEY` | 翻译专用 Key |
| `TMK_TRANS_PROVIDER` | 翻译服务商 |
| `TMK_TRANS_BASE_URL` | 自定义翻译 API 地址 |
| `TMK_TRANS_MODEL` | 自定义翻译模型 |
| `TMK_TTS_ENABLED` | 启用 TTS (`true`/`false`) |
| `TMK_TTS_PROVIDER` | TTS 服务商 |
| `TMK_TTS_API_KEY` | TTS 专用 Key |
| `TMK_TTS_MODEL` | 自定义 TTS 模型 |
| `TMK_TTS_VOICE` | TTS 发音人 |

大多数用户只需设置 `TMK_API_KEY` 和 `TMK_PROVIDER` 即可。

### Provider 预设

指定 `--provider siliconflow` 即自动设置 ASR + 翻译 + TTS 的 Base URL 和 Model。

| Provider | ASR Model | 翻译 Model | TTS Model | 特点 |
|----------|-----------|-----------|-----------|------|
| `groq` | `whisper-large-v3-turbo` | `llama-3.3-70b-versatile` | — | 速度极快 |
| `siliconflow` | `FunAudioLLM/SenseVoiceSmall` | `Qwen/Qwen3-8B` | `CosyVoice2-0.5B` | 中文好，免费 |
| `openai` | `whisper-1` | `gpt-4o-mini` | `tts-1` | 质量最高 |

### 支持的语言

| 代码 | 语言 | ASR | 翻译 |
|------|------|-----|------|
| `zh` | 中文 | ✅ | ✅ |
| `en` | 英文 | ✅ | ✅ |
| `es` | 西班牙文 | ✅ | ✅ |
| `ja` | 日文 | ✅ | ✅ |
| `auto` | 自动检测 | ✅ | — |

---

## 架构设计

### 流式管道

```
麦克风 ──→ [audioChan] ──→ VAD 切句 ──→ [speechChan] ──→ ASR ──→ [textChan] ──→ 翻译(SSE) ──→ Output
 采集        chan []byte    VAD          chan []byte     HTTP      chan *Result    SSE 流式     Console/TTS
 goroutine                goroutine                   goroutine                 goroutine
```

4 个 goroutine 通过 channel 串联，各司其职：

1. **采集**：malgo 回调写入 `audioChan`，每帧 200ms
2. **VAD**：Silero VAD 神经网络检测（优先），能量检测兜底，1.2s 静音判定句子结束
3. **ASR**：将完整句子 PCM 包装为 WAV，multipart POST 到 Whisper API
4. **翻译**：SSE 流式调用 Chat Completions，每收到 delta 即时输出

### 文件管道

```
读文件 → PCM 重采样(16kHz mono) → VAD 分段 → 并发 [ASR + 翻译] → 按序写文件
```

并发处理（默认 4 路信号量），按原始顺序输出结果。

### 接口与扩展点

```go
// asr.Provider — 可接入任意 ASR 服务
type Provider interface {
    Transcribe(ctx context.Context, audioData []byte, lang string) (*Result, error)
}

// translate.Provider — 可接入任意翻译服务
type Provider interface {
    Translate(ctx context.Context, text, srcLang, tgtLang string, onChunk func(string)) error
}

// audio.Capturer — 可替换采集方式（预留 RTC/网络音频扩展）
type Capturer interface {
    Start(onData func([]byte)) error
    Stop() error
}

// output.Output — 可扩展 TTS、WebUI 等输出方式
type Output interface {
    OnSourceText(text string)
    OnTranslatedText(chunk string)
    OnTranslationEnd()
    OnInfo(msg string)
    OnError(msg string)
}
```

### 项目结构

```
Mini_TMK_Agent/
├── cmd/mini-tmk-agent/
│   └── main.go                  # CLI 入口（stream/transcript/web/config）
├── internal/
│   ├── asr/                     # ASR 接口 + Whisper HTTP 实现
│   ├── translate/               # 翻译接口 + LLM SSE 流式翻译
│   ├── tts/                     # TTS 接口 + OpenAI 兼容实现
│   ├── audio/                   # 麦克风采集、文件读取、重采样、VAD（Silero+能量兜底）
│   ├── pipeline/                # 流式管道 + 文件管道
│   ├── config/                  # 配置加载 + .env 支持 + Profile 预设
│   ├── output/                  # 控制台输出 + TTS 装饰器
│   └── web/                     # Web UI（HTTP/WS 服务端 + 嵌入式前端）
│       ├── server.go            #   路由 + go:embed 静态文件
│       ├── handler.go           #   REST API + WebSocket 文件转录
│       ├── ws_stream.go         #   WebSocket 实时同传
│       ├── output.go            #   WebOutput + CollectorOutput
│       ├── capturer.go          #   WebSocketCapturer（浏览器音频）
│       └── static/              #   前端（暗色主题 SPA）
├── go.mod / go.sum
├── Makefile
├── .env.example
└── README.md
```

---

## 开发

```bash
# 自动编译（自动检测 GCC，无 GCC 时使用 nocgo 模式）
make build

# 强制 CGO 模式（麦克风 + Silero VAD，需要 GCC）
make build-full

# 纯 Go 模式（仅文件转录 + Web）
make build-lite

# 单元测试
make test

# 格式化 & 检查
go fmt ./...
go vet ./...
```

<details>
<summary>Windows 安装 GCC（流式同传需要）</summary>

```powershell
# 通过 winget 安装 MSYS2
winget install -e --id MSYS2.MSYS2 --source winget

# 在 MSYS2 中安装 GCC
C:\msys64\usr\bin\bash.exe -lc "pacman -S --noconfirm mingw-w64-x86_64-gcc"

# 加入 PATH
[Environment]::SetEnvironmentVariable("Path", "C:\msys64\mingw64\bin;" + [Environment]::GetEnvironmentVariable("Path", "User"), "User")
```

</details>

### 添加新 Provider

1. 在 `internal/config/config.go` 的 `Profiles` 中添加预设（统一设置 ASR + Trans + TTS）
2. ASR：在 `internal/asr/` 实现 `Provider` 接口
3. 翻译：在 `internal/translate/` 实现 `Provider` 接口
4. TTS：在 `internal/tts/` 实现 `Provider` 接口，在 `NewProvider` 工厂函数中注册

---

## License

MIT
