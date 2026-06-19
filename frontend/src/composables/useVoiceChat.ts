import { ref } from 'vue'
import { chatApi } from '@/api/resources'
import type {
  ChatMessage,
  VoiceChatResult,
  VoiceChatStreamEvent
} from '@/types/domain'
import {
  assistantResultText,
  failedChatResult,
  pendingChatResult,
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
import { notify } from '@/composables/useNotify'
import { useWorkspaceStore } from '@/stores/workspace'
import { useChatStore } from '@/stores/chat'
import { useProjectStore } from '@/stores/project'

const ASR_PROCESSOR_BUFFER_SIZE = 2048
const ASR_PCM_FRAME_BYTES = 1600

type VoiceRealtimeConnection = ReturnType<typeof chatApi.voiceRealtime>
type AudioCaptureController = { stop: () => void }
type VoiceStreamPlayer = {
  push: (chunk: Uint8Array) => void
  finish: () => void
  wait: () => Promise<boolean>
  dispose: () => void
}

export function useVoiceChat() {
  const ws = useWorkspaceStore()
  const chat = useChatStore()
  const project = useProjectStore()

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

  async function toggleVoiceInput() {
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
    if (!chat.selectedSession) return voiceStartWarning('请先创建或选择一个会话')
    if (!ws.context.projectId) return voiceStartWarning('会话缺少项目，请重新选择')
    if (!project.projectDatasources.items.length) return voiceStartWarning('当前项目还没有绑定数据源')
    if (!navigator.mediaDevices?.getUserMedia) return voiceStartWarning('当前浏览器不支持麦克风录音')

    clearVoiceRestartTimer()
    unlockVoicePlayback()
    cleanupVoiceInput()
    voiceBusy.value = true
    ws.loading = true
    voiceTranscript = ''
    voiceTranscriptFinalized = false
    voiceSpeechChunks = []
    voiceSpeechContentType = ''

    const now = Date.now()
    voiceUserMessageId = `voice-user-${now}`
    voiceAssistantPendingId = `voice-assistant-pending-${now}`

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
      voiceBusy.value = false
      ws.loading = false
      voiceLoopActive.value = false
      settleError(error instanceof Error ? error : new Error('无法打开麦克风'))
      return false
    }

    voiceConnection = chatApi.voiceRealtime(ws.context.sessionId, {
      tenant_id: ws.context.tenantId,
      project_id: ws.context.projectId,
      user_id: ws.context.userId,
      selected_datasource_ids: project.projectDatasources.items.map((item) => item.id),
      auto_execute: true,
      max_rows: chat.maxRows
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
        if (!settled && voiceBusy.value) {
          settleError(new Error('实时语音连接已关闭'))
        }
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

  function cleanupVoiceInput() {
    voiceRecording.value = false
    voiceCapture?.stop()
    voiceCapture = null
    voiceStreamPlayer?.dispose()
    voiceStreamPlayer = null
    voiceConnection = null
  }

  function handleVoiceStreamEvent(event: VoiceChatStreamEvent) {
    if (event.stage === 'asr') {
      const text = event.transcript?.text?.trim() || ''
      const terminal = isTerminalVoiceTranscriptEvent(event)
      if (voiceTranscriptFinalized && terminal) return
      if (text && !voiceTranscriptFinalized) {
        voiceTranscript = text
        ensureVoiceUserMessage(voiceTranscript)
      }
      if (terminal && (text || voiceTranscript)) {
        if (text && !voiceTranscriptFinalized) {
          voiceTranscript = text
          ensureVoiceUserMessage(voiceTranscript)
        }
        voiceTranscriptFinalized = true
        stopVoiceInput()
      }
    }
    if (event.stage === 'chat' && event.agent) {
      const pending = ensureVoiceAssistantMessage()
      const question = voiceTranscript || '语音问数'
      const steps = [...(pending.result?.agent.steps || []), event.agent]
      pending.result = pendingChatResult(question, steps)
      pending.content = streamMessageContent(event.agent, pending.content)
    }
    if (event.stage === 'tts' && event.speech) {
      if (event.speech.content_type) voiceSpeechContentType = event.speech.content_type
      if (event.speech.audio_base64_chunk) {
        voiceSpeechChunks.push(event.speech.audio_base64_chunk)
        playVoiceSpeechChunk(event.speech.audio_base64_chunk, voiceSpeechContentType || 'audio/mpeg')
      }
      if (event.speech.done || event.done) {
        voiceStreamPlayer?.finish()
      }
    }
  }

  function isTerminalVoiceTranscriptEvent(event: VoiceChatStreamEvent) {
    const transcript = event.transcript
    return Boolean(event.done || transcript?.done || transcript?.event === 'SentenceEnd' || transcript?.event === 'TranscriptionCompleted')
  }

  function ensureVoiceUserMessage(content: string) {
    let userMessage = chat.messages.find((item) => item.id === voiceUserMessageId)
    if (!userMessage) {
      userMessage = {
        id: voiceUserMessageId,
        role: 'user',
        content,
        createdAt: new Date().toISOString()
      }
      chat.messages.push(userMessage)
      return userMessage
    }
    userMessage.content = content
    return userMessage
  }

  function ensureVoiceAssistantMessage() {
    const pending = chat.messages.find((item) => item.id === voiceAssistantPendingId)
    if (pending) return pending
    const nextMessage: ChatMessage = {
      id: voiceAssistantPendingId,
      role: 'assistant',
      content: '正在理解语音问题并准备问数。',
      createdAt: new Date().toISOString(),
      pending: true,
      result: pendingChatResult(voiceTranscript || '语音问数', [])
    }
    chat.messages.push(nextMessage)
    return nextMessage
  }

  function finishVoiceWithResult(result: VoiceChatResult) {
    const transcript = result.transcript?.text?.trim() || voiceTranscript
    ensureVoiceUserMessage(transcript || '语音输入')

    if (result.chat) chat.latestResult = result.chat
    const assistantMessage: ChatMessage = {
      id: `voice-assistant-${Date.now()}`,
      role: 'assistant',
      content: result.chat ? assistantResultText(result.chat) : result.speech_text || '语音问数已完成。',
      createdAt: new Date().toISOString(),
      result: result.chat || undefined
    }
    replaceVoicePendingMessage(assistantMessage)
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
    const pending = chat.messages.find((item) => item.id === voiceAssistantPendingId)
    const steps = pending?.result?.agent.steps || []
    if (!pending && !voiceTranscript) {
      notify.error(text)
      finishVoiceSession()
      return
    }
    replaceVoicePendingMessage({
      id: `${voiceAssistantPendingId}-failed`,
      role: 'assistant',
      content: `这次语音问数没有完成：${text}`,
      createdAt: new Date().toISOString(),
      result: failedChatResult(voiceTranscript || '语音问数', text, steps)
    })
    notify.error(text)
    finishVoiceSession()
  }

  function replaceVoicePendingMessage(nextMessage: ChatMessage) {
    const pendingIndex = chat.messages.findIndex((item) => item.id === voiceAssistantPendingId)
    if (pendingIndex >= 0) {
      chat.messages.splice(pendingIndex, 1, nextMessage)
    } else {
      chat.messages.push(nextMessage)
    }
  }

  function finishVoiceSession(options: { restart?: boolean } = {}) {
    cleanupVoiceInput()
    voiceBusy.value = false
    ws.loading = false
    if (options.restart && voiceLoopActive.value) {
      scheduleVoiceRestart()
    }
  }

  function scheduleVoiceRestart() {
    clearVoiceRestartTimer()
    voiceRestartTimer = window.setTimeout(() => {
      voiceRestartTimer = null
      if (!voiceLoopActive.value || voiceBusy.value || voiceRecording.value) return
      void startVoiceInput()
    }, 320)
  }

  function clearVoiceRestartTimer() {
    if (voiceRestartTimer === null) return
    window.clearTimeout(voiceRestartTimer)
    voiceRestartTimer = null
  }

  function voiceStartWarning(text: string) {
    voiceLoopActive.value = false
    notify.warning(text)
    return false
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

  async function playVoiceSpeech(result: VoiceChatResult) {
    const contentType = result.speech?.content_type || voiceSpeechContentType || 'audio/mpeg'
    const audioURL = result.speech?.audio_url
    if (voiceSpeechChunks.length) {
      try {
        await playAudioBytes(base64ChunksToArrayBuffer(voiceSpeechChunks), contentType)
      } catch {
        // Ignore playback failures; the chat result is already visible.
      }
      return
    }
    const audioBase64 = result.speech?.audio_base64
    if (audioBase64) {
      await playVoiceBase64(audioBase64, contentType)
      return
    }
    if (!audioURL) return
    await playAudioElement(audioURL)
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

  async function playVoiceBase64(audioBase64: string, contentType: string) {
    await playAudioBytes(base64ToBytes(audioBase64).buffer, contentType)
  }

  async function playAudioBytes(audioData: ArrayBuffer, contentType: string) {
    if (contentType.toLowerCase().includes('pcm')) {
      await playPCM16(audioData)
      return
    }
    const audioContext = ensureVoicePlaybackContext()
    if (!audioContext) return
    if (audioContext.state === 'suspended') {
      await audioContext.resume().catch(() => undefined)
    }
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
    if (audioContext.state === 'suspended') {
      await audioContext.resume().catch(() => undefined)
    }
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
        if (!started) {
          window.setTimeout(() => settle(false), 300)
        }
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
          // Audio nodes may already be disconnected by the browser.
        }
        stream.getTracks().forEach((track) => track.stop())
        void audioContext.close().catch(() => undefined)
      }
    }
  }

  return {
    voiceRecording,
    voiceBusy,
    toggleVoiceInput
  }
}
