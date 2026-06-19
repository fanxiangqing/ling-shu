<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { NButton, NSpin } from 'naive-ui'
import ChatWorkbench from '@/components/ChatWorkbench.vue'
import {
  bootstrapEmbed,
  listEmbedMessages,
  openEmbedVoiceRealtime,
  sendEmbedMessageStream
} from '@/api/embed'
import type {
  ChatMessage,
  DataSourceOption,
  EmbedBootstrapResult,
  SendChatMessageResult,
  VoiceChatResult,
  VoiceChatStreamEvent
} from '@/types/domain'
import {
  assistantResultText,
  failedChatResult,
  parseAgentResultMessage,
  pendingChatResult,
  readableMessageContent,
  streamMessageContent
} from '@/utils/chatResult'
import {
  base64ChunksToArrayBuffer,
  base64ToBytes,
  browserAudioContextCtor,
  createPCMFrameEmitter,
  downsamplePCM,
  encodePCM16,
  exactArrayBuffer,
  mergeByteChunks,
  supportedMediaSourceType
} from '@/utils/audio'

const ASR_PROCESSOR_BUFFER_SIZE = 2048
const ASR_PCM_FRAME_BYTES = 1600

type VoiceRealtimeConnection = ReturnType<typeof openEmbedVoiceRealtime>
type AudioCaptureController = { stop: () => void }
type VoiceStreamPlayer = {
  push: (chunk: Uint8Array) => void
  finish: () => void
  wait: () => Promise<boolean>
  dispose: () => void
}

const loading = ref(false)
const booting = ref(true)
const bootError = ref('')
const bootstrap = ref<EmbedBootstrapResult | null>(null)
const messages = ref<ChatMessage[]>([])
const maxRows = ref(200)
const accessToken = ref('')
const voiceRecording = ref(false)
const voiceBusy = ref(false)
const voiceLoopActive = ref(false)

let voiceConnection: VoiceRealtimeConnection | null = null
let voiceCapture: AudioCaptureController | null = null
let voiceUserMessageId = ''
let voiceAssistantPendingId = ''
let voiceTranscript = ''
let voiceTranscriptFinalized = false
let voiceSpeechChunks: string[] = []
let voiceSpeechContentType = ''
let voicePlaybackContext: AudioContext | null = null
let voiceStreamPlayer: VoiceStreamPlayer | null = null
let voiceRestartTimer: number | null = null

const appId = computed(() => decodeURIComponent(window.location.pathname.split('/').filter(Boolean).pop() || ''))
const sessionKey = computed(() => new URLSearchParams(window.location.search).get('key') || 'default')
const sessionMode = computed(() => new URLSearchParams(window.location.search).get('session_mode') || undefined)
const parentOrigin = computed(() => new URLSearchParams(window.location.search).get('parent_origin') || safeReferrerOrigin())
const projectName = computed(() => bootstrap.value?.app.name || 'Ling-Shu')
const assistantName = computed(() => bootstrap.value?.app.launcher_title || 'Ling-Shu')
const voiceEnabled = computed(() => Boolean(bootstrap.value?.capabilities.realtime_voice))
const datasources = computed<DataSourceOption[]>(() =>
  (bootstrap.value?.datasources || []).map((item) => ({
    id: item.id,
    projectId: bootstrap.value?.project_id || 0,
    name: item.name,
    type: item.db_type,
    dialect: item.db_type,
    role: 'available',
    tableCount: 0,
    syncedAt: item.synced_at || '未同步',
    health: item.status === 'active' ? 'healthy' : 'warning'
  }))
)

onMounted(async () => {
  accessToken.value = readAccessToken()
  if (!appId.value || !accessToken.value) {
    bootError.value = '嵌入参数不完整，请检查 SDK 初始化配置。'
    booting.value = false
    return
  }
  await loadEmbed()
})

onBeforeUnmount(() => {
  voiceLoopActive.value = false
  clearVoiceRestartTimer()
  cleanupVoiceInput()
})

async function loadEmbed() {
  booting.value = true
  bootError.value = ''
  try {
    bootstrap.value = await bootstrapEmbed({
      app_id: appId.value,
      access_token: accessToken.value,
      key: sessionKey.value,
      session_mode: sessionMode.value,
      parent_origin: parentOrigin.value
    })
    const result = await listEmbedMessages(bootstrap.value.session_id, accessToken.value)
    messages.value = result.items.map((item) => {
      const parsed = parseAgentResultMessage(item)
      return {
        id: String(item.id),
        role: item.role === 'user' ? 'user' : 'assistant',
        content: readableMessageContent(item, parsed),
        createdAt: item.created_at || new Date().toISOString(),
        result: parsed || undefined
      }
    })
  } catch (error) {
    bootError.value = error instanceof Error ? error.message : '嵌入助手启动失败。'
  } finally {
    booting.value = false
  }
}

async function ask(question: string) {
  if (!bootstrap.value || loading.value) return
  const now = Date.now()
  const pendingId = `assistant-pending-${now}`
  messages.value.push({ id: `user-${now}`, role: 'user', content: question, createdAt: new Date().toISOString() })
  messages.value.push({
    id: pendingId,
    role: 'assistant',
    content: '正在理解问题、选择数据源并生成查询计划。',
    createdAt: new Date().toISOString(),
    pending: true,
    result: pendingChatResult(question, [])
  })

  loading.value = true
  try {
    const result = await sendEmbedMessageStream(bootstrap.value.session_id, {
      access_token: accessToken.value,
      content: question,
      selected_datasource_ids: datasources.value.map((item) => item.id),
      auto_execute: true,
      max_rows: maxRows.value
    }, (event) => {
      const pending = messages.value.find((item) => item.id === pendingId)
      if (!pending) return
      const steps = [...(pending.result?.agent.steps || []), event]
      pending.result = pendingChatResult(question, steps)
      pending.content = streamMessageContent(event, pending.content)
    })
    replacePendingMessage(pendingId, {
      id: `assistant-${Date.now()}`,
      role: 'assistant',
      content: assistantResultText(result),
      createdAt: new Date().toISOString(),
      result
    })
  } catch (error) {
    const text = error instanceof Error ? error.message : '问数请求失败'
    const pending = messages.value.find((item) => item.id === pendingId)
    replacePendingMessage(pendingId, {
      id: `${pendingId}-failed`,
      role: 'assistant',
      content: `这次问数没有完成：${text}`,
      createdAt: new Date().toISOString(),
      result: failedChatResult(question, text, pending?.result?.agent.steps || [])
    })
  } finally {
    loading.value = false
  }
}

async function toggleVoiceInput() {
  if (!voiceEnabled.value || !bootstrap.value) return
  if (voiceRecording.value) {
    stopVoiceInput()
    return
  }
  if (voiceBusy.value) {
    cancelVoiceInput()
    return
  }
  voiceLoopActive.value = true
  await startVoiceInput()
}

async function startVoiceInput() {
  if (!bootstrap.value) return false
  if (!navigator.mediaDevices?.getUserMedia) {
    showAssistantError('当前浏览器不支持麦克风录音')
    voiceLoopActive.value = false
    return false
  }
  clearVoiceRestartTimer()
  unlockVoicePlayback()
  cleanupVoiceInput()
  voiceBusy.value = true
  loading.value = true
  voiceTranscript = ''
  voiceTranscriptFinalized = false
  voiceSpeechChunks = []
  voiceSpeechContentType = ''

  const now = Date.now()
  voiceUserMessageId = `voice-user-${now}`
  voiceAssistantPendingId = `voice-assistant-pending-${now}`

  let mediaStream: MediaStream
  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true
      }
    })
  } catch (error) {
    voiceLoopActive.value = false
    finishVoiceWithError(error instanceof Error ? error : new Error('无法打开麦克风'))
    return false
  }

  let settled = false
  const settleError = (error: Error) => {
    if (settled) return
    settled = true
    finishVoiceWithError(error)
  }
  const settleSuccess = (result: VoiceChatResult) => {
    if (settled) return
    settled = true
    finishVoiceWithResult(result)
  }

  voiceConnection = openEmbedVoiceRealtime(bootstrap.value.session_id, {
    embed_token: accessToken.value,
    selected_datasource_ids: datasources.value.map((item) => item.id),
    auto_execute: true,
    max_rows: maxRows.value
  }, {
    onOpen: () => {
      try {
        voiceRecording.value = true
        voiceCapture = createPcmAudioCapture(mediaStream, (chunk) => {
          voiceConnection?.sendAudio(chunk)
        })
      } catch (error) {
        mediaStream.getTracks().forEach((track) => track.stop())
        settleError(error instanceof Error ? error : new Error('麦克风采集初始化失败'))
        voiceConnection?.abort()
      }
    },
    onEvent: handleVoiceStreamEvent,
    onResult: settleSuccess,
    onError: settleError,
    onClose: () => {
      if (!settled && voiceBusy.value) settleError(new Error('实时语音连接已关闭'))
    }
  })
  return true
}

function stopVoiceInput() {
  if (!voiceRecording.value) return
  voiceRecording.value = false
  voiceCapture?.stop()
  voiceCapture = null
  if (!voiceTranscript.trim()) {
    voiceLoopActive.value = false
    voiceConnection?.abort()
    finishVoiceSession()
    return
  }
  voiceConnection?.stop()
}

function cancelVoiceInput() {
  voiceLoopActive.value = false
  clearVoiceRestartTimer()
  voiceConnection?.abort()
  finishVoiceSession()
}

function handleVoiceStreamEvent(event: VoiceChatStreamEvent) {
  if (event.stage === 'asr') {
    const text = event.transcript?.text?.trim() || ''
    const terminal = Boolean(event.done || event.transcript?.done || event.transcript?.event === 'SentenceEnd' || event.transcript?.event === 'TranscriptionCompleted')
    if (voiceTranscriptFinalized && terminal) return
    if (text && !voiceTranscriptFinalized) {
      voiceTranscript = text
      ensureVoiceUserMessage(text)
    }
    if (terminal && (text || voiceTranscript)) {
      if (text && !voiceTranscriptFinalized) ensureVoiceUserMessage(text)
      voiceTranscriptFinalized = true
      stopVoiceInput()
    }
  }
  if (event.stage === 'chat' && event.agent) {
    const pending = ensureVoiceAssistantMessage()
    const steps = [...(pending.result?.agent.steps || []), event.agent]
    pending.result = pendingChatResult(voiceTranscript || '语音问数', steps)
    pending.content = streamMessageContent(event.agent, pending.content)
  }
  if (event.stage === 'tts' && event.speech) {
    if (event.speech.content_type) voiceSpeechContentType = event.speech.content_type
    if (event.speech.audio_base64_chunk) {
      voiceSpeechChunks.push(event.speech.audio_base64_chunk)
      playVoiceSpeechChunk(event.speech.audio_base64_chunk, voiceSpeechContentType || 'audio/mpeg')
    }
    if (event.speech.done || event.done) voiceStreamPlayer?.finish()
  }
}

function finishVoiceWithResult(result: VoiceChatResult) {
  const transcript = result.transcript?.text?.trim() || voiceTranscript
  ensureVoiceUserMessage(transcript || '语音输入')
  replacePendingMessage(voiceAssistantPendingId, {
    id: `voice-assistant-${Date.now()}`,
    role: 'assistant',
    content: result.chat ? assistantResultText(result.chat) : result.speech_text || '语音问数已完成。',
    createdAt: new Date().toISOString(),
    result: result.chat || undefined
  })
  void finishVoiceResultAndContinue(result)
}

async function finishVoiceResultAndContinue(result: VoiceChatResult) {
  try {
    await finishVoiceSpeechPlayback(result)
  } finally {
    finishVoiceSession({ restart: voiceLoopActive.value })
  }
}

function finishVoiceWithError(error: Error) {
  voiceLoopActive.value = false
  const text = error.message || '语音问数失败'
  const pending = messages.value.find((item) => item.id === voiceAssistantPendingId)
  if (pending || voiceTranscript) {
    replacePendingMessage(voiceAssistantPendingId, {
      id: `${voiceAssistantPendingId}-failed`,
      role: 'assistant',
      content: `这次语音问数没有完成：${text}`,
      createdAt: new Date().toISOString(),
      result: failedChatResult(voiceTranscript || '语音问数', text, pending?.result?.agent.steps || [])
    })
  } else {
    showAssistantError(text)
  }
  finishVoiceSession()
}

function finishVoiceSession(options: { restart?: boolean } = {}) {
  cleanupVoiceInput()
  voiceBusy.value = false
  loading.value = false
  if (options.restart && voiceLoopActive.value) scheduleVoiceRestart()
}

function scheduleVoiceRestart() {
  clearVoiceRestartTimer()
  voiceRestartTimer = window.setTimeout(() => {
    voiceRestartTimer = null
    if (!voiceLoopActive.value || voiceBusy.value || voiceRecording.value) return
    void startVoiceInput()
  }, 360)
}

function clearVoiceRestartTimer() {
  if (voiceRestartTimer === null) return
  window.clearTimeout(voiceRestartTimer)
  voiceRestartTimer = null
}

function cleanupVoiceInput() {
  voiceRecording.value = false
  voiceCapture?.stop()
  voiceCapture = null
  voiceStreamPlayer?.dispose()
  voiceStreamPlayer = null
  voiceConnection = null
}

function ensureVoiceUserMessage(content: string) {
  let message = messages.value.find((item) => item.id === voiceUserMessageId)
  if (!message) {
    message = { id: voiceUserMessageId, role: 'user', content, createdAt: new Date().toISOString() }
    messages.value.push(message)
    return message
  }
  message.content = content
  return message
}

function ensureVoiceAssistantMessage() {
  const pending = messages.value.find((item) => item.id === voiceAssistantPendingId)
  if (pending) return pending
  const nextMessage: ChatMessage = {
    id: voiceAssistantPendingId,
    role: 'assistant',
    content: '正在理解语音问题并准备问数。',
    createdAt: new Date().toISOString(),
    pending: true,
    result: pendingChatResult(voiceTranscript || '语音问数', [])
  }
  messages.value.push(nextMessage)
  return nextMessage
}

function replacePendingMessage(id: string, nextMessage: ChatMessage) {
  const index = messages.value.findIndex((item) => item.id === id)
  if (index >= 0) {
    messages.value.splice(index, 1, nextMessage)
  } else {
    messages.value.push(nextMessage)
  }
}

async function playVoiceSpeech(result: VoiceChatResult) {
  const contentType = result.speech?.content_type || voiceSpeechContentType || 'audio/mpeg'
  if (voiceSpeechChunks.length) {
    await playAudioBytes(base64ChunksToArrayBuffer(voiceSpeechChunks), contentType).catch(() => undefined)
    return
  }
  if (result.speech?.audio_base64) {
    await playAudioBytes(base64ToBytes(result.speech.audio_base64).buffer, contentType).catch(() => undefined)
    return
  }
  if (result.speech?.audio_url) {
    await playAudioElement(result.speech.audio_url)
  }
}

function playVoiceSpeechChunk(audioBase64Chunk: string, contentType: string) {
  const chunk = base64ToBytes(audioBase64Chunk)
  if (!chunk.byteLength) return
  if (!voiceStreamPlayer) {
    voiceStreamPlayer = createVoiceStreamPlayer(contentType)
  }
  voiceStreamPlayer.push(chunk)
}

async function finishVoiceSpeechPlayback(result: VoiceChatResult) {
  const player = voiceStreamPlayer
  if (player) {
    player.finish()
    const streamed = await player.wait().catch(() => false)
    player.dispose()
    voiceStreamPlayer = null
    if (streamed) return
  }
  await playVoiceSpeech(result)
}

function unlockVoicePlayback() {
  const AudioContextCtor = browserAudioContextCtor()
  if (!AudioContextCtor) return
  if (!voicePlaybackContext || voicePlaybackContext.state === 'closed') {
    voicePlaybackContext = new AudioContextCtor()
  }
  if (voicePlaybackContext.state === 'suspended') {
    void voicePlaybackContext.resume().catch(() => undefined)
  }
}

async function playAudioBytes(audioData: ArrayBuffer, contentType: string) {
  if (contentType.toLowerCase().includes('pcm')) {
    await playPCM16(audioData)
    return
  }
  const audioContext = ensureVoicePlaybackContext()
  if (!audioContext) return
  if (audioContext.state === 'suspended') await audioContext.resume().catch(() => undefined)
  if (audioContext.state === 'suspended') return
  const buffer = await audioContext.decodeAudioData(audioData.slice(0))
  const source = audioContext.createBufferSource()
  source.buffer = buffer
  source.connect(audioContext.destination)
  const ended = new Promise<void>((resolve) => {
    source.onended = () => resolve()
  })
  source.start()
  await ended
}

async function playPCM16(audioData: ArrayBuffer) {
  const audioContext = ensureVoicePlaybackContext()
  if (!audioContext) return
  if (audioContext.state === 'suspended') await audioContext.resume().catch(() => undefined)
  if (audioContext.state === 'suspended') return
  const pcm = new Int16Array(audioData)
  const audioBuffer = audioContext.createBuffer(1, pcm.length, 16000)
  const channel = audioBuffer.getChannelData(0)
  for (let index = 0; index < pcm.length; index += 1) {
    channel[index] = Math.max(-1, Math.min(1, pcm[index] / 32768))
  }
  const source = audioContext.createBufferSource()
  source.buffer = audioBuffer
  source.connect(audioContext.destination)
  const ended = new Promise<void>((resolve) => {
    source.onended = () => resolve()
  })
  source.start()
  await ended
}

async function playAudioElement(src: string) {
  const audio = new Audio(src)
  try {
    await audio.play()
  } catch {
    return
  }
  await new Promise<void>((resolve) => {
    const finish = () => {
      audio.removeEventListener('ended', finish)
      audio.removeEventListener('error', finish)
      resolve()
    }
    audio.addEventListener('ended', finish)
    audio.addEventListener('error', finish)
  })
}

function ensureVoicePlaybackContext() {
  if (voicePlaybackContext && voicePlaybackContext.state !== 'closed') return voicePlaybackContext
  const AudioContextCtor = browserAudioContextCtor()
  if (!AudioContextCtor) return null
  voicePlaybackContext = new AudioContextCtor()
  return voicePlaybackContext
}

function createVoiceStreamPlayer(contentType: string): VoiceStreamPlayer {
  if (contentType.toLowerCase().includes('pcm')) {
    return createPCMVoiceStreamPlayer()
  }
  const mediaSourceType = supportedMediaSourceType(contentType)
  if (mediaSourceType) {
    return createMediaSourceVoiceStreamPlayer(mediaSourceType)
  }
  return createBufferedVoiceStreamPlayer(contentType)
}

function createMediaSourceVoiceStreamPlayer(contentType: string): VoiceStreamPlayer {
  const mediaSource = new MediaSource()
  const audio = new Audio()
  const objectURL = URL.createObjectURL(mediaSource)
  const queue: Uint8Array[] = []
  let sourceBuffer: SourceBuffer | null = null
  let finished = false
  let disposed = false
  let hasAudio = false
  let started = false
  let resolved = false
  let resolveDone: (played: boolean) => void = () => undefined
  const done = new Promise<boolean>((resolve) => {
    resolveDone = resolve
  })

  const settle = (played: boolean) => {
    if (resolved) return
    resolved = true
    resolveDone(played)
  }
  const tryPlay = () => {
    const playPromise = audio.play()
    if (playPromise) {
      void playPromise.then(() => {
        started = true
      }).catch(() => undefined)
    }
  }
  const appendNext = () => {
    if (disposed || !sourceBuffer || sourceBuffer.updating) return
    if (queue.length) {
      const next = queue.shift()
      if (!next) return
      try {
        sourceBuffer.appendBuffer(exactArrayBuffer(next))
        tryPlay()
      } catch {
        settle(started)
      }
      return
    }
    if (finished && mediaSource.readyState === 'open') {
      try {
        mediaSource.endOfStream()
      } catch {
        settle(started)
      }
      if (!started) window.setTimeout(() => settle(false), 300)
    }
  }

  audio.src = objectURL
  audio.preload = 'auto'
  audio.addEventListener('playing', () => {
    started = true
  })
  audio.addEventListener('ended', () => settle(hasAudio))
  audio.addEventListener('error', () => settle(started))
  mediaSource.addEventListener('sourceopen', () => {
    if (disposed) return
    try {
      sourceBuffer = mediaSource.addSourceBuffer(contentType)
    } catch {
      settle(false)
      return
    }
    sourceBuffer.addEventListener('updateend', appendNext)
    sourceBuffer.addEventListener('error', () => settle(started))
    appendNext()
    tryPlay()
  }, { once: true })

  return {
    push(chunk: Uint8Array) {
      if (disposed || resolved) return
      hasAudio = true
      queue.push(chunk)
      appendNext()
    },
    finish() {
      finished = true
      appendNext()
      if (!hasAudio) settle(false)
    },
    wait() {
      return done
    },
    dispose() {
      disposed = true
      try {
        audio.pause()
        audio.removeAttribute('src')
        audio.load()
      } catch {
        // Audio element cleanup is best-effort.
      }
      URL.revokeObjectURL(objectURL)
      settle(started)
    }
  }
}

function createPCMVoiceStreamPlayer(): VoiceStreamPlayer {
  let nextStartTime = 0
  let hasAudio = false
  let finished = false
  let lastEnded: Promise<void> = Promise.resolve()
  let resolveDone: (played: boolean) => void = () => undefined
  const done = new Promise<boolean>((resolve) => {
    resolveDone = resolve
  })

  return {
    push(chunk: Uint8Array) {
      const audioContext = ensureVoicePlaybackContext()
      if (!audioContext || !chunk.byteLength) return
      void audioContext.resume().catch(() => undefined)
      const buffer = exactArrayBuffer(chunk)
      const pcm = new Int16Array(buffer.slice(0, Math.floor(buffer.byteLength / 2) * 2))
      if (!pcm.length) return
      const audioBuffer = audioContext.createBuffer(1, pcm.length, 16000)
      const channel = audioBuffer.getChannelData(0)
      for (let index = 0; index < pcm.length; index += 1) {
        channel[index] = Math.max(-1, Math.min(1, pcm[index] / 32768))
      }
      const source = audioContext.createBufferSource()
      source.buffer = audioBuffer
      source.connect(audioContext.destination)
      const startAt = Math.max(audioContext.currentTime + 0.02, nextStartTime)
      nextStartTime = startAt + audioBuffer.duration
      lastEnded = new Promise<void>((resolve) => {
        source.onended = () => resolve()
      })
      hasAudio = true
      source.start(startAt)
    },
    finish() {
      if (finished) return
      finished = true
      void lastEnded.then(() => resolveDone(hasAudio))
    },
    wait() {
      return done
    },
    dispose() {
      resolveDone(hasAudio)
    }
  }
}

function createBufferedVoiceStreamPlayer(contentType: string): VoiceStreamPlayer {
  const chunks: Uint8Array[] = []
  let finished = false
  let resolveDone: (played: boolean) => void = () => undefined
  const done = new Promise<boolean>((resolve) => {
    resolveDone = resolve
  })
  return {
    push(chunk: Uint8Array) {
      if (!finished) chunks.push(chunk)
    },
    finish() {
      if (finished) return
      finished = true
      void playAudioBytes(mergeByteChunks(chunks), contentType)
        .then(() => resolveDone(chunks.length > 0))
        .catch(() => resolveDone(false))
    },
    wait() {
      return done
    },
    dispose() {
      finished = true
      resolveDone(false)
    }
  }
}

function createPcmAudioCapture(stream: MediaStream, onChunk: (chunk: ArrayBuffer) => void): AudioCaptureController {
  const AudioContextCtor = browserAudioContextCtor()
  if (!AudioContextCtor) throw new Error('当前浏览器不支持实时音频采集')
  let audioContext: AudioContext
  try {
    audioContext = new AudioContextCtor({ sampleRate: 16000 })
  } catch {
    audioContext = new AudioContextCtor()
  }
  const source = audioContext.createMediaStreamSource(stream)
  const processor = audioContext.createScriptProcessor(ASR_PROCESSOR_BUFFER_SIZE, 1, 1)
  const mutedOutput = audioContext.createGain()
  const frameEmitter = createPCMFrameEmitter(ASR_PCM_FRAME_BYTES, onChunk)
  mutedOutput.gain.value = 0
  const inputSampleRate = audioContext.sampleRate
  processor.onaudioprocess = (event) => {
    if (!voiceRecording.value) return
    const input = event.inputBuffer.getChannelData(0)
    const pcm = encodePCM16(downsamplePCM(input, inputSampleRate, 16000))
    frameEmitter.push(pcm)
  }
  source.connect(processor)
  processor.connect(mutedOutput)
  mutedOutput.connect(audioContext.destination)
  void audioContext.resume().catch(() => undefined)
  return {
    stop() {
      frameEmitter.flush()
      processor.onaudioprocess = null
      try {
        processor.disconnect()
        mutedOutput.disconnect()
        source.disconnect()
      } catch {
        // Audio nodes may already be disconnected.
      }
      stream.getTracks().forEach((track) => track.stop())
      void audioContext.close().catch(() => undefined)
    }
  }
}

function readAccessToken() {
  const hash = window.location.hash.startsWith('#') ? window.location.hash.slice(1) : window.location.hash
  const hashParams = new URLSearchParams(hash)
  return hashParams.get('token') || new URLSearchParams(window.location.search).get('token') || ''
}

function safeReferrerOrigin() {
  try {
    return document.referrer ? new URL(document.referrer).origin : ''
  } catch {
    return ''
  }
}

function showAssistantError(text: string) {
  messages.value.push({
    id: `assistant-error-${Date.now()}`,
    role: 'assistant',
    content: text,
    createdAt: new Date().toISOString()
  })
}
</script>

<template>
  <section class="embed-chat-page">
    <div v-if="booting" class="embed-state">
      <NSpin size="medium" />
    </div>
    <div v-else-if="bootError" class="embed-state error">
      <h1>助手暂时不可用</h1>
      <p>{{ bootError }}</p>
      <NButton size="small" @click="loadEmbed">重试</NButton>
    </div>
    <ChatWorkbench
      v-else-if="bootstrap"
      :messages="messages"
      :datasources="datasources"
      :session-id="bootstrap.session_id"
      :project-name="projectName"
      :session-title="bootstrap.app.launcher_title || '智能问数'"
      :auto-execute="true"
      :max-rows="maxRows"
      :loading="loading"
      :voice-recording="voiceRecording"
      :voice-busy="voiceBusy"
      :voice-enabled="voiceEnabled"
      :assistant-name="assistantName"
      :welcome-message="bootstrap.app.welcome_message"
      @ask="ask"
      @voice-toggle="toggleVoiceInput"
      @update:max-rows="maxRows = $event"
    />
  </section>
</template>
