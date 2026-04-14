# Mini TMK Agent

同声传译 Agent — 麦克风实时翻译 & 音频文件转录 & Web UI，支持中/英/西/日四种语言互译。

## 3 分钟快速上手

> 前提：已安装 [Go 1.21+](https://go.dev/dl/)

```bash
# 1. 克隆并编译
git clone <repo-url> && cd Mini_TMK_Agent
go build -tags nocgo -o mini-tmk-agent ./cmd/mini-tmk-agent

# 2. 配置 API Key（免费获取，见下方说明）
cp .env.example .env
```

编辑 `.env`，填入你的 Key：

```env
TMK_ASR_API_KEY=sk-your-key-here
TMK_TRANS_API_KEY=sk-your-key-here
```

```bash
# 3. 转录一个音频文件试试
./mini-tmk-agent transcript --file your-audio.wav --output result.txt
```

看到结果输出就成功了！

### 免费获取 API Key

推荐使用 **SiliconFlow**，注册即送免费额度，中文识别效果好：

| 平台 | 注册地址 | 特点 |
|------|---------|------|
| **SiliconFlow** | [cloud.siliconflow.cn](https://cloud.siliconflow.cn/) | 中文好，免费额度充足 |
| **Groq** | [console.groq.com](https://console.groq.com/) | 速度极快，免费额度充足 |

两个平台的 API Key 都可以同时用于 ASR 和翻译，`.env` 中填同一个 Key 即可。

---

## 使用方法

### 文件转录

```bash
# 基本用法：转录中文音频，翻译为英文
./mini-tmk-agent transcript --file input.wav --output output.txt

# 自动检测语言 → 翻译为中文
./mini-tmk-agent transcript --file speech.mp3 -s auto -t zh

# 指定 provider
./mini-tmk-agent transcript --file audio.wav -s en -t zh \
    --asr-provider groq --trans-provider siliconflow
```

支持 `.wav`、`.mp3`、`.pcm` 格式。

### TTS 语音合成（可选）

转录结果可自动生成目标语言的语音文件（MP3）：

```bash
# 转录 + TTS 语音输出（使用 SiliconFlow CosyVoice2）
./mini-tmk-agent transcript --file input.wav --output output.txt \
    --tts --tts-provider siliconflow
# 生成 output.txt + output.mp3

# 使用 OpenAI TTS
./mini-tmk-agent transcript --file input.wav --output output.txt \
    --tts --tts-provider openai --tts-voice nova

# 流式同传也支持 TTS（每段独立保存为 segment_001.mp3 ...）
./mini-tmk-agent stream -s zh -t en --tts --tts-provider siliconflow
```

也可通过环境变量启用（在 `.env` 中配置）：

```env
TMK_TTS_ENABLED=true
TMK_TTS_PROVIDER=siliconflow
TMK_TTS_API_KEY=your-key
```

### Web UI（浏览器可视化界面）

```bash
# 启动 Web 服务（默认端口 8080）
./mini-tmk-agent web

# 指定端口
./mini-tmk-agent web -p 3000
```

浏览器打开 `http://localhost:8080`，支持：

- **文件转录**：拖拽上传音频文件，流式显示转录+翻译结果，带进度条
- **实时同传**：浏览器麦克风采集，WebSocket 实时传输 ASR+翻译结果
- **TTS 播放**：开启后翻译结果带播放按钮，一键收听译文语音
- **配置管理**：在线切换 Provider、语言对，设置 API Key

技术栈：Go embed 嵌入前端 + gorilla/websocket，零额外构建工具链。

### 流式同传（需要麦克风 + GCC）

```bash
# 编译带麦克风支持的版本（需要 GCC）
go build -o mini-tmk-agent ./cmd/mini-tmk-agent

# 启动实时翻译（中 → 英）
./mini-tmk-agent stream --source-lang zh --target-lang en

# 英 → 中，verbose 模式同时显示原文
./mini-tmk-agent stream -s en -t zh -v
```

按 `Ctrl+C` 停止。

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

---

## 命令参考

### `web` — Web UI 服务

```bash
mini-tmk-agent web [flags]
```

| Flag | 短 | 默认值 | 说明 |
|------|----|--------|------|
| `--port` | `-p` | `8080` | Web 服务端口 |

### `stream` — 流式同传

```bash
mini-tmk-agent stream [flags]
```

| Flag | 短 | 默认值 | 说明 |
|------|----|--------|------|
| `--source-lang` | `-s` | `auto` | 源语言 (`auto`/`zh`/`en`/`es`/`ja`) |
| `--target-lang` | `-t` | `zh` | 目标语言 |
| `--asr-provider` | | 配置文件 | ASR 服务商 (`groq`/`siliconflow`/`openai`) |
| `--trans-provider` | | 配置文件 | 翻译服务商 |
| `--tts` | | `false` | 启用 TTS 语音合成 |
| `--tts-provider` | | 配置文件 | TTS 服务商 (`siliconflow`/`openai`) |
| `--tts-voice` | | 配置文件 | TTS 发音人 |
| `--verbose` | `-v` | `false` | 显示原文和详细信息 |

```bash
mini-tmk-agent transcript --file <audio-file> [--output <output-file>] [flags]
```

| Flag | 短 | 默认值 | 说明 |
|------|----|--------|------|
| `--file` | | （必填） | 输入音频文件 (`.wav`/`.mp3`/`.pcm`) |
| `--output` | | 无 | 输出文件路径（不指定则仅控制台输出） |
| `--source-lang` | `-s` | `auto` | 源语言 |
| `--target-lang` | `-t` | `zh` | 目标语言 |
| `--asr-provider` | | 配置文件 | ASR 服务商 |
| `--trans-provider` | | 配置文件 | 翻译服务商 |
| `--tts` | | `false` | 启用 TTS 语音合成 |
| `--tts-provider` | | 配置文件 | TTS 服务商 (`siliconflow`/`openai`) |
| `--tts-voice` | | 配置文件 | TTS 发音人 |
| `--verbose` | `-v` | `false` | 显示详细信息 |

### `version` — 版本信息

```bash
mini-tmk-agent version
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
| `TMK_ASR_API_KEY` | ASR 服务 API Key |
| `TMK_ASR_PROVIDER` | ASR 服务商 (`groq`/`siliconflow`/`openai`)，大小写不敏感 |
| `TMK_ASR_BASE_URL` | 自定义 ASR API 地址 |
| `TMK_ASR_MODEL` | 自定义 ASR 模型 |
| `TMK_TRANS_API_KEY` | 翻译服务 API Key |
| `TMK_TRANS_PROVIDER` | 翻译服务商 |
| `TMK_TRANS_BASE_URL` | 自定义翻译 API 地址 |
| `TMK_TRANS_MODEL` | 自定义翻译模型 |
| `TMK_TTS_ENABLED` | 启用 TTS 语音合成 (`true`/`false`) |
| `TMK_TTS_PROVIDER` | TTS 服务商 (`siliconflow`/`openai`) |
| `TMK_TTS_API_KEY` | TTS 服务 API Key（可与 ASR/翻译共用） |
| `TMK_TTS_MODEL` | 自定义 TTS 模型 |
| `TMK_TTS_VOICE` | TTS 发音人 |

### Provider 预设

指定 `--asr-provider groq` 即自动使用对应的 Base URL 和 Model。

| Provider | ASR Model | 翻译 Model | TTS Model | 特点 |
|----------|-----------|-----------|-----------|------|
| `groq` | `whisper-large-v3-turbo` | `llama-3.3-70b-versatile` | — | 速度极快 |
| `siliconflow` | `FunAudioLLM/SenseVoiceSmall` | `Qwen/Qwen3-8B` | `CosyVoice2-0.5B` | 中文好，免费 |
| `openai` | `whisper-1` | `gpt-4o-mini` | `tts-1` | 质量最高 |

三个 Provider 均兼容 OpenAI API 格式，改 Key 和 Provider 名即可切换。

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
│   └── main.go                  # CLI 入口（stream/transcript/web）
├── internal/
│   ├── asr/                     # ASR 接口 + Whisper HTTP 实现
│   ├── translate/               # 翻译接口 + LLM SSE 流式翻译
│   ├── tts/                     # TTS 接口 + OpenAI 兼容实现
│   ├── audio/                   # 麦克风采集、文件读取、重采样、VAD（Silero+能量兜底）
│   ├── pipeline/                # 流式管道 + 文件管道
│   ├── config/                  # 配置加载 + .env 支持
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
# 单元测试（不需要 GCC）
go test -tags nocgo ./...

# 编译（带麦克风支持，需要 GCC）
go build -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent

# 编译（仅文件模式，不需要 GCC）
go build -tags nocgo -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent

# 格式化 & 检查
go fmt ./...
go vet ./...
```

### 添加新 Provider

1. ASR：在 `internal/asr/` 实现 `Provider` 接口，在 `config.go` 的 `ProviderDefaults` 中添加预设
2. 翻译：在 `internal/translate/` 实现 `Provider` 接口，在 `TransDefaults` 中添加预设
3. TTS：在 `internal/tts/` 实现 `Provider` 接口，在 `TTSDefaults` 中添加预设，在 `NewProvider` 工厂函数中注册

---

## License

MIT
