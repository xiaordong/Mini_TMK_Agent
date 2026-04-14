// Mini TMK Agent Web UI
(function () {
  'use strict';

  // ===== 状态 =====
  let currentMode = 'file';
  let ws = null;
  let audioCtx = null;
  let mediaStream = null;
  let processor = null;
  let isRecording = false;

  // ===== DOM 引用 =====
  const $ = (sel) => document.querySelector(sel);
  const $$ = (sel) => document.querySelectorAll(sel);

  const statusEl = $('#status');
  const footerStatus = $('#footerStatus');
  const fileMode = $('#fileMode');
  const streamMode = $('#streamMode');
  const uploadArea = $('#uploadArea');
  const fileInput = $('#fileInput');
  const fileProgress = $('#fileProgress');
  const progressBar = $('#progressBar');
  const progressText = $('#progressText');
  const progressPct = $('#progressPct');
  const fileResults = $('#fileResults');
  const streamResults = $('#streamResults');
  const streamStart = $('#streamStart');
  const streamStop = $('#streamStop');
  const settingsModal = $('#settingsModal');

  // ===== 初始化 =====
  function init() {
    bindModeSwitch();
    bindFileUpload();
    bindStreamControls();
    bindSettings();
    loadConfig();
  }

  // ===== 模式切换 =====
  function bindModeSwitch() {
    $$('.mode-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        $$('.mode-btn').forEach((b) => b.classList.remove('active'));
        btn.classList.add('active');
        currentMode = btn.dataset.mode;

        fileMode.classList.toggle('hidden', currentMode !== 'file');
        streamMode.classList.toggle('hidden', currentMode !== 'stream');
      });
    });
  }

  // ===== 文件上传 =====
  function bindFileUpload() {
    uploadArea.addEventListener('click', () => fileInput.click());

    uploadArea.addEventListener('dragover', (e) => {
      e.preventDefault();
      uploadArea.classList.add('dragover');
    });
    uploadArea.addEventListener('dragleave', () => {
      uploadArea.classList.remove('dragover');
    });
    uploadArea.addEventListener('drop', (e) => {
      e.preventDefault();
      uploadArea.classList.remove('dragover');
      if (e.dataTransfer.files.length > 0) {
        uploadFile(e.dataTransfer.files[0]);
      }
    });

    fileInput.addEventListener('change', () => {
      if (fileInput.files.length > 0) {
        uploadFile(fileInput.files[0]);
      }
    });
  }

  async function uploadFile(file) {
    fileResults.innerHTML = '';
    fileProgress.classList.remove('hidden');
    progressText.textContent = '读取文件: ' + file.name;
    progressBar.style.width = '0%';
    progressPct.textContent = '';

    // 先读取文件为 ArrayBuffer
    const arrayBuffer = await file.arrayBuffer();

    const source = $('#sourceLang').value;
    const target = $('#targetLang').value;
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${proto}//${location.host}/ws/transcript?source=${source}&target=${target}&filename=${encodeURIComponent(file.name)}`;

    const transcriptWs = new WebSocket(wsUrl);

    transcriptWs.onopen = () => {
      // 发送文件二进制数据
      transcriptWs.send(arrayBuffer);
      progressText.textContent = '上传完成，处理中...';
      progressBar.style.width = '5%';
    };

    // 当前正在接收的结果卡片
    let currentCard = null;
    let currentTranslationEl = null;
    let currentTTSData = null;
    let segmentTotal = 0;
    let segmentDone = 0;

    transcriptWs.onmessage = (e) => {
      const msg = JSON.parse(e.data);

      switch (msg.type) {
        case 'source':
          currentCard = createResultCard(msg.content, '');
          currentTranslationEl = currentCard.querySelector('.result-translation');
          currentTTSData = null;
          fileResults.appendChild(currentCard);
          break;

        case 'translated':
          if (currentTranslationEl) {
            currentTranslationEl.textContent += msg.content;
          }
          break;

        case 'end':
          if (currentCard && currentTTSData) {
            addPlayButton(currentCard, currentTTSData);
          }
          currentCard = null;
          currentTranslationEl = null;
          break;

        case 'tts_audio':
          currentTTSData = msg.content;
          if (currentCard && !currentTranslationEl) {
            addPlayButton(currentCard, currentTTSData);
          }
          break;

        case 'progress':
          progressText.textContent = msg.content;
          // 从 "检测到 N 个语音段" 消息中提取总数
          var m1 = msg.content.match(/检测到\s*(\d+)\s*个语音段/);
          if (m1) segmentTotal = parseInt(m1[1], 10);
          // 从 "完成 X/N 段" 消息提取已完成数（原子计数，单调递增）
          var m2 = msg.content.match(/完成\s*(\d+)\/(\d+)\s*段/);
          if (m2) {
            var done = parseInt(m2[1], 10);
            var total = parseInt(m2[2], 10);
            if (done > segmentDone) segmentDone = done;
            if (total > segmentTotal) segmentTotal = total;
            updateFileProgress(segmentDone, segmentTotal);
          }
          break;

        case 'info':
          progressText.textContent = msg.content;
          break;

        case 'error':
          fileResults.insertAdjacentHTML('beforeend',
            `<div class="error-msg">${msg.content}</div>`);
          break;

        case 'done':
          fileProgress.classList.add('hidden');
          if (segmentTotal > 0) {
            progressText.textContent = '完成';
            progressBar.style.width = '100%';
            progressPct.textContent = '100%';
          }
          break;
      }
    };

    transcriptWs.onerror = () => {
      fileProgress.classList.add('hidden');
      fileResults.innerHTML = '<div class="error-msg">连接失败</div>';
    };

    transcriptWs.onclose = () => {
      fileProgress.classList.add('hidden');
    };
  }

  function updateFileProgress(done, total) {
    if (total <= 0) return;
    const pct = Math.min(100, Math.round((done / total) * 100));
    progressBar.style.width = pct + '%';
    progressPct.textContent = pct + '%';
    progressText.textContent = `处理中 ${done}/${total} 段`;
  }

  // ===== 实时模式 =====
  function bindStreamControls() {
    streamStart.addEventListener('click', startStream);
    streamStop.addEventListener('click', stopStream);
  }

  async function startStream() {
    try {
      mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: { sampleRate: 16000, channelCount: 1, echoCancellation: true },
      });
    } catch (err) {
      alert('无法访问麦克风: ' + err.message);
      return;
    }

    const source = $('#sourceLang').value;
    const target = $('#targetLang').value;
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${proto}//${location.host}/ws/stream?source=${source}&target=${target}`);

    ws.onopen = () => {
      statusEl.textContent = '● 录音中';
      statusEl.className = 'status connected';
      footerStatus.textContent = '录音中...';
      streamStart.classList.add('hidden');
      streamStop.classList.remove('hidden');
      streamResults.innerHTML = '';
      isRecording = true;

      startAudioCapture();
    };

    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      handleStreamMessage(msg);
    };

    ws.onclose = () => {
      cleanupStream();
    };

    ws.onerror = () => {
      cleanupStream();
    };
  }

  function startAudioCapture() {
    audioCtx = new AudioContext({ sampleRate: 16000 });
    const source = audioCtx.createMediaStreamSource(mediaStream);

    processor = audioCtx.createScriptProcessor(4096, 1, 1);
    processor.onaudioprocess = (e) => {
      if (!isRecording || !ws || ws.readyState !== WebSocket.OPEN) return;

      const float32 = e.inputBuffer.getChannelData(0);
      const int16 = new Int16Array(float32.length);
      for (let i = 0; i < float32.length; i++) {
        const s = Math.max(-1, Math.min(1, float32[i]));
        int16[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
      }
      ws.send(int16.buffer);
    };

    source.connect(processor);
    processor.connect(audioCtx.destination);
  }

  function stopStream() {
    isRecording = false;
    if (ws) ws.close();
    cleanupStream();
  }

  function cleanupStream() {
    isRecording = false;
    if (processor) { processor.disconnect(); processor = null; }
    if (audioCtx) { audioCtx.close(); audioCtx = null; }
    if (mediaStream) {
      mediaStream.getTracks().forEach((t) => t.stop());
      mediaStream = null;
    }

    statusEl.textContent = '● 已就绪';
    statusEl.className = 'status connected';
    footerStatus.textContent = '就绪';
    streamStart.classList.remove('hidden');
    streamStop.classList.add('hidden');
  }

  let currentStreamCard = null;
  let currentTranslationEl = null;
  let currentTTSData = null;

  function handleStreamMessage(msg) {
    switch (msg.type) {
      case 'source':
        currentStreamCard = createResultCard(msg.content, '');
        currentTranslationEl = currentStreamCard.querySelector('.result-translation');
        currentTTSData = null;
        streamResults.appendChild(currentStreamCard);
        break;

      case 'translated':
        if (currentTranslationEl) {
          currentTranslationEl.textContent += msg.content;
        }
        break;

      case 'end':
        if (currentStreamCard && currentTTSData) {
          addPlayButton(currentStreamCard, currentTTSData);
        }
        currentStreamCard = null;
        currentTranslationEl = null;
        break;

      case 'tts_audio':
        currentTTSData = msg.content;
        if (currentStreamCard && !currentTranslationEl) {
          addPlayButton(currentStreamCard, currentTTSData);
          currentStreamCard = null;
        }
        break;

      case 'info':
        footerStatus.textContent = msg.content;
        break;

      case 'error':
        footerStatus.textContent = '错误: ' + msg.content;
        break;
    }
  }

  // ===== 结果卡片 =====
  function createResultCard(source, translation) {
    const card = document.createElement('div');
    card.className = 'result-card';

    if (source) {
      const srcEl = document.createElement('div');
      srcEl.className = 'result-source';
      srcEl.textContent = source;
      card.appendChild(srcEl);
    }

    const transEl = document.createElement('div');
    transEl.className = 'result-translation';
    transEl.textContent = translation || '';
    card.appendChild(transEl);

    return card;
  }

  function addPlayButton(card, audioBase64) {
    // 避免重复添加
    if (card.querySelector('.btn-play')) return;
    const actions = document.createElement('div');
    actions.className = 'result-actions';

    const btn = document.createElement('button');
    btn.className = 'btn-play';
    btn.textContent = '▶ 播放';
    btn.addEventListener('click', () => {
      const audio = new Audio('data:audio/mp3;base64,' + audioBase64);
      audio.play();
      btn.textContent = '⏸ 播放中';
      audio.addEventListener('ended', () => { btn.textContent = '▶ 播放'; });
    });
    actions.appendChild(btn);
    card.appendChild(actions);
  }

  // ===== 配置 =====
  async function loadConfig() {
    try {
      const resp = await fetch('/api/config');
      const cfg = await resp.json();

      fillProviderSelect($('#asrProvider'), ['groq', 'siliconflow', 'openai'], cfg.asr_provider);
      fillProviderSelect($('#transProvider'), ['groq', 'siliconflow', 'openai'], cfg.trans_provider);

      $('#ttsEnabled').checked = cfg.tts_enabled;

      if (cfg.source_lang) $('#sourceLang').value = cfg.source_lang;
      if (cfg.target_lang) $('#targetLang').value = cfg.target_lang;

      // API Key 状态：显示来源（.env 文件加载 / 未设置）
      updateKeyStatus('asrKeyStatus', cfg.asr_api_key_set);
      updateKeyStatus('transKeyStatus', cfg.trans_api_key_set);
      updateKeyStatus('ttsKeyStatus', cfg.tts_api_key_set);
    } catch (err) {
      console.error('加载配置失败:', err);
      statusEl.textContent = '● 连接失败';
      statusEl.className = 'status disconnected';
      return;
    }

    // 配置加载成功，服务可用
    statusEl.textContent = '● 已就绪';
    statusEl.className = 'status connected';
  }

  function updateKeyStatus(elId, isSet) {
    const el = $('#' + elId);
    if (isSet) {
      el.textContent = '已通过配置文件/环境变量加载';
      el.style.color = 'var(--success)';
      // 对应的输入框设为 disabled + placeholder
      const inputId = elId.replace('KeyStatus', 'ApiKey');
      const input = $('#' + inputId);
      if (input) {
        input.placeholder = '已配置，留空保持不变';
        input.disabled = false;
      }
    } else {
      el.textContent = '未设置，请在下方输入';
      el.style.color = 'var(--error)';
    }
  }

  function fillProviderSelect(el, options, current) {
    el.innerHTML = '';
    options.forEach((opt) => {
      const option = document.createElement('option');
      option.value = opt;
      option.textContent = opt;
      if (opt === current) option.selected = true;
      el.appendChild(option);
    });
  }

  function bindProviderChange() {
    $('#asrProvider').addEventListener('change', syncConfig);
    $('#transProvider').addEventListener('change', syncConfig);
    $('#ttsEnabled').addEventListener('change', syncConfig);
    $('#sourceLang').addEventListener('change', syncConfig);
    $('#targetLang').addEventListener('change', syncConfig);
  }

  async function syncConfig() {
    const body = {
      asr_provider: $('#asrProvider').value,
      trans_provider: $('#transProvider').value,
      tts_enabled: $('#ttsEnabled').checked,
      source_lang: $('#sourceLang').value,
      target_lang: $('#targetLang').value,
    };
    try {
      await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
    } catch (err) {
      console.error('同步配置失败:', err);
    }
  }

  // ===== 设置弹窗 =====
  function bindSettings() {
    $('#settingsBtn').addEventListener('click', () => {
      settingsModal.classList.remove('hidden');
    });
    $('#settingsClose').addEventListener('click', () => {
      settingsModal.classList.add('hidden');
    });
    settingsModal.addEventListener('click', (e) => {
      if (e.target === settingsModal) settingsModal.classList.add('hidden');
    });

    $('#saveSettings').addEventListener('click', async () => {
      const body = {};
      const asrKey = $('#asrApiKey').value;
      const transKey = $('#transApiKey').value;
      const ttsKey = $('#ttsApiKey').value;
      if (asrKey) body.asr_api_key = asrKey;
      if (transKey) body.trans_api_key = transKey;
      if (ttsKey) body.tts_api_key = ttsKey;

      try {
        await fetch('/api/config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        settingsModal.classList.add('hidden');
        // 清空输入框
        $('#asrApiKey').value = '';
        $('#transApiKey').value = '';
        $('#ttsApiKey').value = '';
        loadConfig();
      } catch (err) {
        alert('保存失败: ' + err.message);
      }
    });
  }

  // ===== 启动 =====
  document.addEventListener('DOMContentLoaded', () => {
    init();
    bindProviderChange();
  });
})();
