export function exactArrayBuffer(bytes: Uint8Array) {
  const copy = new Uint8Array(bytes.byteLength)
  copy.set(bytes)
  return copy.buffer
}

export function mergeByteChunks(chunks: Uint8Array[]) {
  const total = chunks.reduce((sum, chunk) => sum + chunk.byteLength, 0)
  const merged = new Uint8Array(total)
  let offset = 0
  for (const chunk of chunks) {
    merged.set(chunk, offset)
    offset += chunk.byteLength
  }
  return merged.buffer
}

export function base64ToBytes(value: string) {
  const binary = atob(value.trim())
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return bytes
}

export function base64ChunksToArrayBuffer(chunks: string[]) {
  const parts = chunks.map(base64ToBytes).filter((part) => part.byteLength > 0)
  const total = parts.reduce((sum, part) => sum + part.byteLength, 0)
  const merged = new Uint8Array(total)
  let offset = 0
  for (const part of parts) {
    merged.set(part, offset)
    offset += part.byteLength
  }
  return merged.buffer
}

export function browserAudioContextCtor() {
  return window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
}

export function supportedMediaSourceType(contentType: string) {
  if (typeof MediaSource === 'undefined') return ''
  const normalized = contentType.toLowerCase().split(';')[0].trim()
  const candidates = normalized.includes('mpeg') || normalized.includes('mp3')
    ? ['audio/mpeg', 'audio/mpeg; codecs="mp3"']
    : [contentType]
  return candidates.find((candidate) => MediaSource.isTypeSupported(candidate)) || ''
}

export function downsamplePCM(input: Float32Array, inputSampleRate: number, outputSampleRate: number) {
  if (inputSampleRate <= outputSampleRate) return input
  const ratio = inputSampleRate / outputSampleRate
  const outputLength = Math.floor(input.length / ratio)
  const output = new Float32Array(outputLength)
  let inputOffset = 0
  for (let outputOffset = 0; outputOffset < outputLength; outputOffset += 1) {
    const nextInputOffset = Math.floor((outputOffset + 1) * ratio)
    let sum = 0
    let count = 0
    for (let index = inputOffset; index < nextInputOffset && index < input.length; index += 1) {
      sum += input[index]
      count += 1
    }
    output[outputOffset] = count > 0 ? sum / count : 0
    inputOffset = nextInputOffset
  }
  return output
}

export function encodePCM16(input: Float32Array) {
  const buffer = new ArrayBuffer(input.length * 2)
  const view = new DataView(buffer)
  for (let index = 0; index < input.length; index += 1) {
    const sample = Math.max(-1, Math.min(1, input[index]))
    view.setInt16(index * 2, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true)
  }
  return buffer
}

export function createPCMFrameEmitter(frameBytes: number, onChunk: (chunk: ArrayBuffer) => void) {
  let pending = new Uint8Array(0)
  return {
    push(buffer: ArrayBuffer) {
      if (!buffer.byteLength) return
      const incoming = new Uint8Array(buffer)
      const merged = new Uint8Array(pending.byteLength + incoming.byteLength)
      merged.set(pending)
      merged.set(incoming, pending.byteLength)
      let offset = 0
      while (offset + frameBytes <= merged.byteLength) {
        onChunk(exactArrayBuffer(merged.slice(offset, offset + frameBytes)))
        offset += frameBytes
      }
      pending = merged.slice(offset)
    },
    flush() {
      if (!pending.byteLength) return
      onChunk(exactArrayBuffer(pending))
      pending = new Uint8Array(0)
    }
  }
}
