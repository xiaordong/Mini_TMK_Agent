# Mini TMK Agent

同声传译 CLI 工具 — 麦克风实时翻译 & 音频文件转录，支持中/英/西/日四种语言互译。

## 功能一览

| 功能 | 说明 |
|------|------|
| 流式同传 | 麦克风实时采集 → VAD 切句 → ASR 识别 → SSE 流式翻译 → 控制台输出 |
| 文件转录 | 读取 WAV/MP3/PCM → 自动分段 → 逐段 ASR + 翻译 → 写入文本文件 |
| 多 Provider | Groq（快+免费）、SiliconFlow（中文好+免费）、OpenAI（质量最高） |
| 开箱即用 | `.env` 文件或环境变量配置，一个 Key 即可开始 |

---

## 快速开始

### 1. 环境要求

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | ≥ 1.21 | 编译运行 |
| GCC (MinGW-w64) | 任意 | 麦克风采集需要 CGO（文件转录模式不需要） |

<details>
<summary>Windows 安装 GCC（点击展开）</summary>

```powershell
# 方式一：winget 安装 MSYS2
winget install -e --id MSYS2.MSYS2 --source winget

# 安装完成后，在 MSYS2 shell 中执行
C:\msys64\usr\bin\bash.exe -lc "pacman -S --noconfirm mingw-w64-x86_64-gcc"

# 将 MinGW 加入系统 PATH
[Environment]::SetEnvironmentVariable("Path", "C:\msys64\mingw64\bin;" + [Environment]::GetEnvironmentVariable("Path", "User"), "User")
```

</details>

### 2. 编译

```bash
# 克隆
git clone <repo-url> && cd Mini_TMK_Agent

# 带麦克风支持（需要 GCC）
go build -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent

# 仅文件转录模式（不需要 GCC）
go build -tags nocgo -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent
```

### 3. 配置 API Key

```bash
# 方式一：.env 文件（推荐）
cp .env.example .env
# 编辑 .env，填入你的 API Key
```

`.env` 内容示例：

```env
TMK_ASR_API_KEY=sk-your-key-here
TMK_ASR_PROVIDER=siliconflow
TMK_TRANS_API_KEY=sk-your-key-here
TMK_TRANS_PROVIDER=siliconflow
```

```bash
# 方式二：环境变量
export TMK_ASR_API_KEY="sk-your-key"
export TMK_TRANS_API_KEY="sk-your-key"
```

免费 API Key 获取：
- **[SiliconFlow](https://cloud.siliconflow.cn/)** — 推荐，中文识别优秀，免费额度充足
- **[Groq](https://console.groq.com/)** — 速度极快，免费额度充足

### 4. 运行

```bash
# 流式同传（中 → 英）
./mini-tmk-agent stream --source-lang zh --target-lang en

# 文件转录
./mini-tmk-agent transcript --file input.mp3 --output output.txt

# verbose 模式（同时显示原文和译文）
./mini-tmk-agent stream -s en -t zh -v

# 指定 provider
./mini-tmk-agent transcript --file audio.wav -s en -t zh \
    --asr-provider siliconflow --trans-provider groq
```

---

## 命令参考

### `stream` — 流式同传

通过麦克风实时采集音频，进行语音识别和流式翻译。

```bash
mini-tmk-agent stream [flags]
```

| Flag | 短 | 默认值 | 说明 |
|------|----|--------|------|
| `--source-lang` | `-s` | `auto` | 源语言 (`auto`/`zh`/`en`/`es`/`ja`) |
| `--target-lang` | `-t` | `zh` | 目标语言 (`zh`/`en`/`es`/`ja`) |
| `--asr-provider` | | 配置文件 | ASR 服务商 (`groq`/`siliconflow`/`openai`) |
| `--trans-provider` | | 配置文件 | 翻译服务商 (`groq`/`siliconflow`/`openai`) |
| `--verbose` | `-v` | `false` | 显示原文和详细信息 |

按 `Ctrl+C` 停止。

### `transcript` — 文件转录

读取音频文件，分段进行语音识别和翻译，结果输出到文本文件。

```bash
mini-tmk-agent transcript --file <audio-file> [--output <output-file>] [flags]
```

| Flag | 短 | 默认值 | 说明 |
|------|----|--------|------|
| `--file` | | （必填） | 输入音频文件路径 (`.wav`/`.mp3`/`.pcm`) |
| `--output` | | 无 | 输出文件路径（不指定则仅控制台输出） |
| `--source-lang` | `-s` | `auto` | 源语言 |
| `--target-lang` | `-t` | `zh` | 目标语言 |
| `--asr-provider` | | 配置文件 | ASR 服务商 |
| `--trans-provider` | | 配置文件 | 翻译服务商 |
| `--verbose` | `-v` | `false` | 显示详细信息 |

### `version` — 版本信息

```bash
mini-tmk-agent version
```

---

## 配置详情

配置优先级（高 → 低）：

```
命令行 flags  >  环境变量  >  .env 文件  >  ~/.mini-tmk-agent.yaml  >  默认值
```

### 环境变量

| 变量名 | 说明 |
|--------|------|
| `TMK_ASR_API_KEY` | ASR 服务 API Key |
| `TMK_ASR_PROVIDER` | ASR 服务商 (`groq`/`siliconflow`/`openai`)，大小写不敏感 |
| `TMK_ASR_BASE_URL` | 自定义 ASR API 地址（覆盖默认） |
| `TMK_ASR_MODEL` | 自定义 ASR 模型（覆盖默认） |
| `TMK_TRANS_API_KEY` | 翻译服务 API Key |
| `TMK_TRANS_PROVIDER` | 翻译服务商 |
| `TMK_TRANS_BASE_URL` | 自定义翻译 API 地址 |
| `TMK_TRANS_MODEL` | 自定义翻译模型 |

### Provider 预设

指定 `--asr-provider groq` 即自动使用下面对应的 Base URL 和 Model，无需手动配置。

| Provider | ASR Model | 翻译 Model | Base URL | 特点 |
|----------|-----------|-----------|----------|------|
| `groq` | `whisper-large-v3-turbo` | `llama-3.3-70b-versatile` | `api.groq.com/openai/v1` | 速度极快，免费额度 |
| `siliconflow` | `FunAudioLLM/SenseVoiceSmall` | `Qwen/Qwen3-8B` | `api.siliconflow.cn/v1` | 中文识别好，免费 |
| `openai` | `whisper-1` | `gpt-4o-mini` | `api.openai.com/v1` | 质量最高 |

三个 Provider 均兼容 OpenAI API 格式，只需改 Key 和 Provider 名即可切换。

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
麦克风 ──→ [audioChan] ──→ VAD 切句 ──→ [speechChan] ──→ ASR ──→ [textChan] ──→ 翻译(SSE) ──→ 控制台
 采集        chan []byte    VAD          chan []byte     HTTP      chan *Result    SSE 流式     Output
 goroutine                goroutine                   goroutine                 goroutine
```

4 个 goroutine 通过 channel 串联，各司其职：

1. **采集 goroutine**：malgo 回调写入 `audioChan`，每帧 200ms
2. **VAD goroutine**：能量检测状态机，自动校准底噪阈值，1.2s 静音判定句子结束
3. **ASR goroutine**：将完整句子 PCM 包装为 WAV，multipart POST 到 Whisper API
4. **翻译 goroutine**：SSE 流式调用 Chat Completions，每收到 delta 即时输出

### 文件管道

```
读文件 → PCM 重采样(16kHz mono) → 按 30s 分段 → 逐段 [ASR → 翻译] → 写文件
```

顺序处理，无并发，保证输出顺序。

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
│   └── main.go                  # CLI 入口，cobra 命令 + 管道组装
├── internal/
│   ├── asr/
│   │   ├── asr.go               # Provider 接口 + 工厂
│   │   └── whisper.go           # OpenAI Whisper API HTTP 实现
│   ├── translate/
│   │   ├── translate.go         # Provider 接口 + 工厂
│   │   └── llm.go               # Chat Completions SSE 流式翻译
│   ├── audio/
│   │   ├── capture.go           # malgo 麦克风采集 (CGO)
│   │   ├── capture_nocgo.go     # 无 CGO 时的 stub
│   │   ├── reader.go            # WAV/MP3/PCM 文件读取
│   │   ├── resample.go          # 线性插值重采样 + stereo→mono
│   │   ├── vad.go               # 能量检测 VAD 状态机
│   │   └── wav.go               # WAV header 解析
│   ├── pipeline/
│   │   ├── stream.go            # 流式同传管道（4 goroutine）
│   │   └── file.go              # 文件转录管道
│   ├── config/
│   │   └── config.go            # viper 配置加载 + .env 支持
│   └── output/
│       └── console.go           # ANSI 彩色控制台输出
├── go.mod / go.sum
├── Makefile
├── .env.example
└── README.md
```

---

## 开发

### 运行测试

```bash
# 单元测试（不需要 GCC）
go test -tags nocgo ./...

# 带 CGO 的完整测试（需要 GCC）
go test ./...

# 集成测试（需要真实 API Key，使用 build tag 控制）
go test -tags integration ./...
```

### 常用命令

```bash
# 编译（带麦克风支持）
go build -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent

# 编译（无 CGO，仅文件模式）
go build -tags nocgo -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent

# 格式化
go fmt ./...

# 代码检查
go vet ./...
```

### 添加新 Provider

1. ASR：在 `internal/asr/` 实现 `Provider` 接口，在 `internal/config/config.go` 的 `ProviderDefaults` 中添加预设
2. 翻译：在 `internal/translate/` 实现 `Provider` 接口，在 `TransDefaults` 中添加预设
3. 所有 Provider 均兼容 OpenAI API 格式，通常只需改 Base URL 和 Model

### 添加新输出方式（TTS / WebUI）

实现 `output.Output` 接口，在 CLI 入口替换 `output.NewConsoleOutput()` 即可。

---

## License

MIT
