# Mini TMK Agent

同声传译 Agent — 麦克风实时翻译 & 音频文件转录 & Web UI，支持中/英/西/日四种语言互译。

三种使用方式：

- **`stream`** — 麦克风实时同传（对着麦克风说话，实时出翻译）
- **`transcript`** — 音频文件转录（上传 wav/mp3，输出双语文本）
- **`web`** — Web UI（浏览器操作，支持以上所有功能 + TTS 语音播放）

底层调用 OpenAI 兼容的 Whisper（语音识别）+ Chat Completions（翻译）+ TTS API，已内置 SiliconFlow / Groq / OpenAI 三个服务商预设，也可自定义接入任意兼容 API。

---

## 快速开始

### 1. 安装依赖

- [Go 1.21+](https://go.dev/dl/)
- GCC（可选）— `stream` 命令（麦克风采集）需要 CGO，`transcript` 和 `web` 不需要

### 2. 克隆并编译

```bash
# 克隆项目
git clone <repo-url>
cd Mini_TMK_Agent

# 下载 Go 依赖
go mod download

# 编译（二选一）
# 有 GCC（支持麦克风同传）
go build -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent

# 没有 GCC（仅文件转录 + Web）
go build -tags nocgo -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent
```

> Windows 用户编译后产物是 `mini-tmk-agent.exe`，后续命令中 `mini-tmk-agent` 替换为 `mini-tmk-agent.exe` 即可。

### 3. 配置

只需两步：选服务商、填 API Key。

```bash
# 选择服务商（三选一）
mini-tmk-agent config set provider siliconflow

# 填写 API Key
mini-tmk-agent config set api-key sk-your-key-here
```

**推荐 SiliconFlow**：注册送免费额度，中文识别效果好。其他可选：

| 服务商 | 注册地址 | ASR 模型 | 翻译模型 | 特点 |
|--------|---------|----------|---------|------|
| **SiliconFlow** | [cloud.siliconflow.cn](https://cloud.siliconflow.cn/) | SenseVoiceSmall | Qwen3-8B | 中文好，免费额度充足 |
| **Groq** | [console.groq.com](https://console.groq.com/) | whisper-large-v3-turbo | llama-3.3-70b | 速度极快，免费额度充足 |
| **OpenAI** | [platform.openai.com](https://platform.openai.com/) | whisper-1 | gpt-4o-mini | 质量最高 |

### 4. 使用

```bash
# 转录音频文件（最简单，不需要麦克风）
mini-tmk-agent transcript input.wav

# 实时麦克风同传（中文 → 英文）
mini-tmk-agent stream -s zh -t en

# 启动 Web UI
mini-tmk-agent web
# 浏览器打开 http://localhost:8080
```

---

## 使用自定义 / 第三方 API

项目使用 OpenAI 兼容协议，可以接入任何提供 Whisper + Chat Completions 接口的服务。例如 DeepSeek、月之暗面、自建 API 等。

### 方式一：使用已知服务商预设

```bash
mini-tmk-agent config set provider siliconflow
mini-tmk-agent config set api-key sk-xxx
```

设完 provider 后，ASR 和翻译的 Base URL / 模型名会自动填充，无需额外配置。

### 方式二：自定义 Base URL 和模型

如果你用的服务商不在预设中，或想用聚合 API（一个地址走通 ASR + 翻译），可以手动覆盖：

```bash
# 先选一个接近的 provider 预设作为基础
mini-tmk-agent config set provider siliconflow
mini-tmk-agent config set api-key sk-xxx

# 再覆盖具体的 Base URL 和模型
mini-tmk-agent config set asr-base-url https://your-api.com/v1
mini-tmk-agent config set asr-model whisper-large-v3
mini-tmk-agent config set trans-base-url https://your-api.com/v1
mini-tmk-agent config set trans-model deepseek-chat
```

### 方式三：ASR 和翻译用不同服务商

```bash
mini-tmk-agent config set provider siliconflow   # 默认用 SiliconFlow
mini-tmk-agent config set api-key sk-sf-xxx

# 翻译单独指定用 DeepSeek
mini-tmk-agent config set trans-base-url https://api.deepseek.com/v1
mini-tmk-agent config set trans-model deepseek-chat
mini-tmk-agent config set trans-api-key sk-ds-xxx  # 翻译专用 Key
```

### 方式四：纯环境变量（不用 config set）

在项目目录下创建 `.env` 文件：

```env
# 最简配置：统一服务商 + Key
TMK_PROVIDER=siliconflow
TMK_API_KEY=sk-xxx

# 或者完全自定义
TMK_ASR_BASE_URL=https://your-api.com/v1
TMK_ASR_MODEL=whisper-large-v3
TMK_ASR_API_KEY=sk-xxx
TMK_TRANS_BASE_URL=https://your-api.com/v1
TMK_TRANS_MODEL=deepseek-chat
TMK_TRANS_API_KEY=sk-xxx
```

也可以直接设系统环境变量，效果相同。

### API 兼容性要求

| 服务 | 接口协议 | 说明 |
|------|---------|------|
| ASR（语音识别） | `POST /v1/audio/transcriptions` | Whisper 兼容，multipart/form-data 上传音频 |
| 翻译 | `POST /v1/chat/completions` | OpenAI Chat Completions 兼容，支持 SSE 流式 |
| TTS（可选） | `POST /v1/audio/speech` | OpenAI TTS 兼容，返回音频流 |

只要你的 API 提供商支持以上接口，就可以直接使用。

---

## 命令详解

### `transcript` — 音频文件转录

```bash
# 基本用法（输出到终端）
mini-tmk-agent transcript input.wav

# 指定输出文件
mini-tmk-agent transcript input.wav output.txt

# 指定语言对
mini-tmk-agent transcript speech.mp3 -s en -t zh

# 使用其他服务商
mini-tmk-agent transcript input.wav --provider groq
```

支持 `.wav`、`.mp3`、`.pcm` 格式。不需要 GCC，任何平台都能用。

### `stream` — 实时麦克风同传

```bash
# 中文 → 英文
mini-tmk-agent stream -s zh -t en

# 自动检测源语言 → 中文
mini-tmk-agent stream -s auto -t zh

# 显示原文
mini-tmk-agent stream -s zh -t en -v
```

按 `Ctrl+C` 停止。**需要 GCC**（编译时带 CGO），否则会报错。

<details>
<summary>Windows 安装 GCC</summary>

```powershell
# 安装 MSYS2
winget install -e --id MSYS2.MSYS2

# 在 MSYS2 中安装 GCC
C:\msys64\usr\bin\bash.exe -lc "pacman -S --noconfirm mingw-w64-x86_64-gcc"

# 加入 PATH（重启终端生效）
[Environment]::SetEnvironmentVariable("Path", "C:\msys64\mingw64\bin;" + [Environment]::GetEnvironmentVariable("Path", "User"), "User")
```

</details>

### `web` — Web UI

```bash
mini-tmk-agent web          # 默认 8080 端口
mini-tmk-agent web -p 3000  # 指定端口
```

浏览器打开 `http://localhost:8080`，功能包括：

- **文件转录**：拖拽上传音频，流式显示双语结果
- **实时同传**：浏览器麦克风采集，WebSocket 实时显示
- **TTS 播放**：翻译结果带播放按钮，收听译文语音
- **在线配置**：切换服务商、语言对、API Key

### `config` — 配置管理

```bash
# 查看当前配置
mini-tmk-agent config show

# 检查配置和环境是否就绪
mini-tmk-agent config check

# 设置配置项
mini-tmk-agent config set <key> <value>
```

所有可配置项：

| Key | 说明 | 示例 |
|-----|------|------|
| `provider` | 统一服务商 | `siliconflow` / `groq` / `openai` |
| `api-key` | 统一 API Key | `sk-xxx` |
| `asr-base-url` | ASR API 地址（覆盖预设） | `https://api.example.com/v1` |
| `asr-model` | ASR 模型名 | `whisper-large-v3` |
| `asr-api-key` | ASR 专用 Key | `sk-xxx` |
| `trans-base-url` | 翻译 API 地址（覆盖预设） | `https://api.deepseek.com/v1` |
| `trans-model` | 翻译模型名 | `deepseek-chat` |
| `trans-api-key` | 翻译专用 Key | `sk-xxx` |
| `tts` | 启用 TTS | `true` / `false` |
| `tts-base-url` | TTS API 地址 | `https://api.siliconflow.cn/v1` |
| `tts-model` | TTS 模型名 | `FunAudioLLM/CosyVoice2-0.5B` |
| `tts-voice` | TTS 发音人 | `FunAudioLLM/CosyVoice2-0.5B:alex` |

---

## TTS 语音合成（可选）

启用后，翻译结果会额外生成语音文件。

```bash
# 启用 TTS
mini-tmk-agent config set tts true

# 转录时自动生成译文语音（output.txt + output.mp3）
mini-tmk-agent transcript input.wav output.txt

# 实时同传时也会播放译文语音
mini-tmk-agent stream -s zh -t en
```

TTS 需要 API 支持 `/v1/audio/speech` 接口。SiliconFlow 和 OpenAI 支持，Groq 不支持。

---

## 配置优先级

```
命令行 flags  >  环境变量  >  .env 文件  >  ~/.mini-tmk-agent.yaml  >  默认值
```

配置文件位置：`~/.mini-tmk-agent.yaml`（用户主目录下）。

环境变量前缀为 `TMK_`，例如 `TMK_API_KEY`、`TMK_PROVIDER`。

---

## 支持的语言

| 代码 | 语言 | 语音识别 | 翻译 |
|------|------|---------|------|
| `zh` | 中文 | ✅ | ✅ |
| `en` | 英文 | ✅ | ✅ |
| `es` | 西班牙文 | ✅ | ✅ |
| `ja` | 日文 | ✅ | ✅ |
| `auto` | 自动检测 | ✅ | — |

翻译的目标语言不能设为 `auto`。

---

## 架构

### 流式管道（stream / web 实时同传）

```
麦克风 ──→ [audioChan] ──→ VAD 切句 ──→ [speechChan] ──→ ASR ──→ [textChan] ──→ translateDispatch
 采集        chan []byte    (Silero)     chan []byte     Whisper     chan *Result    (分配 index,
                                                                                      OnSourceText)
                                                                                      │
                                                                                [taskChan] (缓冲=N)
                                                                                 ┌──┴──┐
                                                                              worker ×N (并行翻译)
                                                                                 └──┬──┘
                                                                                [resultChan]
                                                                                      │
                                                                                outputLoop (串行输出)
```

- 翻译采用 worker pool（默认 3 路，上限 5 路），并行处理多段话
- 每个翻译任务带 30s 超时，防止网络假死
- 所有 Output 调用在单 goroutine 中串行执行，无需加锁

### 文件管道（transcript）

```
读文件 → PCM 重采样(16kHz mono) → VAD 分段 → 并发 [ASR + 翻译] → 按序输出
```

并发处理（默认 4 路信号量），结果按原始顺序输出。

### 接口扩展

项目采用接口设计，可以自行接入任意 ASR / 翻译 / TTS 服务：

```go
// ASR — internal/asr/asr.go
type Provider interface {
    Transcribe(ctx context.Context, audioData []byte, lang string) (*Result, error)
}

// 翻译 — internal/translate/translate.go
type Provider interface {
    Translate(ctx context.Context, text, srcLang, tgtLang string, onChunk func(string)) error
}

// TTS — internal/tts/tts.go
type Provider interface {
    Synthesize(ctx context.Context, text, lang string) ([]byte, error)
}
```

### 项目结构

```
Mini_TMK_Agent/
├── cmd/mini-tmk-agent/
│   └── main.go                  # CLI 入口
├── internal/
│   ├── asr/                     # ASR 接口 + Whisper HTTP 实现
│   ├── translate/               # 翻译接口 + LLM SSE 流式翻译
│   ├── tts/                     # TTS 接口 + OpenAI 兼容实现
│   ├── audio/                   # 麦克风采集、文件读取、重采样、VAD
│   ├── pipeline/                # 流式管道 + 文件管道
│   ├── config/                  # 配置加载 + Provider 预设
│   ├── output/                  # 控制台输出 + TTS 装饰器
│   └── web/                     # Web UI（HTTP/WS + 嵌入式前端）
├── go.mod / go.sum
├── .env.example
└── README.md
```

---

## 开发

```bash
# CGO 模式（麦克风 + Silero VAD，需要 GCC）
go build -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent

# 纯 Go 模式（仅文件转录 + Web，不需要 GCC）
go build -tags nocgo -ldflags "-s -w" -o mini-tmk-agent ./cmd/mini-tmk-agent

# 单元测试
go test ./...

# 格式化
go fmt ./...
go vet ./...
```

有 Makefile 可用 `make build`，但不是必须的，Windows 用户直接用 `go build` 即可。

### 添加新 Provider

1. 在 `internal/config/config.go` 的 `Profiles` 中添加预设
2. 在对应包中实现 `Provider` 接口（ASR / 翻译 / TTS 各自独立）
3. 在工厂函数中注册

---

## License

[AGPL-3.0](LICENSE)
