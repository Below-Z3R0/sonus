/**
 * sonus-read — reads Hermes assistant deltas aloud through Vox.
 *
 * Vox must be running at 127.0.0.1:9102 and Kokoro loaded:
 *   voxctl service load kokoro-82m-onnx
 *
 * The plugin cuts text at sentence boundaries as message.delta events arrive
 * and POSTs each sentence to /v1/speech/stream. Audio is played as PCM16.
 */

import { atom, host, useValue, cn, haptic } from '@hermes/plugin-sdk'
import { jsx } from 'react/jsx-runtime'

const ID = 'sonus-read'
const VOX = 'http://127.0.0.1:9102/v1/speech/stream'
const $enabled = atom(false)

let storage = null
let audio = null
let nextAt = 0
let pending = ''
let chain = Promise.resolve()

function sentenceSplit(text) {
  const out = []
  let rest = text
  let match
  const re = /[^.!?…]+[.!?…]+(?:\s+|$)/
  while ((match = re.exec(rest)) !== null) {
    const s = match[0].trim()
    if (s) out.push(s)
    rest = rest.slice(match.index + match[0].length)
    if (match[0].length === 0) break
  }
  return { sentences: out, rest }
}

function pcm16ToFloat(bytes, leftover) {
  const all = new Uint8Array(leftover.length + bytes.length)
  all.set(leftover)
  all.set(bytes, leftover.length)
  const usable = all.length - (all.length % 2)
  const view = new DataView(all.buffer, all.byteOffset, usable)
  const samples = new Float32Array(usable / 2)
  for (let i = 0; i < samples.length; i++) {
    samples[i] = view.getInt16(i * 2, true) / 32768
  }
  return { samples, leftover: all.slice(usable) }
}

async function playSentence(text) {
  if (!text.trim()) return
  audio ||= new AudioContext()
  if (audio.state === 'suspended') await audio.resume()
  const res = await fetch(VOX, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text, lang: 'auto', voice: '' })
  })
  if (!res.ok || !res.body) {
    throw new Error(`Vox ${res.status}`)
  }
  const rate = Number(res.headers.get('X-Sample-Rate') || 24000)
  const reader = res.body.getReader()
  let leftover = new Uint8Array()
  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    const chunk = pcm16ToFloat(value, leftover)
    leftover = chunk.leftover
    if (!chunk.samples.length) continue
    const buffer = audio.createBuffer(1, chunk.samples.length, rate)
    buffer.copyToChannel(chunk.samples, 0)
    const src = audio.createBufferSource()
    src.buffer = buffer
    src.connect(audio.destination)
    const start = Math.max(audio.currentTime, nextAt)
    src.start(start)
    nextAt = start + buffer.duration
  }
}

function enqueue(sentence) {
  chain = chain.then(() => playSentence(sentence)).catch(err => {
    host.notify({ kind: 'error', message: `sonus-read: ${err.message}` })
  })
}

function onDelta(event) {
  if (!$enabled.get()) return
  const data = event?.data ?? event?.payload ?? event
  const text = data?.delta ?? data?.text ?? event?.text ?? ''
  if (!text) return
  pending += text
  const { sentences, rest } = sentenceSplit(pending)
  pending = rest
  for (const sentence of sentences) enqueue(sentence)
}

function flush() {
  if (pending.trim()) enqueue(pending.trim())
  pending = ''
}

function Chip() {
  const on = useValue($enabled)
  return jsx('button', {
    type: 'button',
    className: cn(
      'inline-flex h-full items-center gap-1 px-1.5 text-[0.6875rem] transition-colors',
      on ? 'text-(--ui-accent)' : 'text-(--ui-text-tertiary) hover:text-foreground'
    ),
    onClick: () => {
      const next = !$enabled.get()
      $enabled.set(next)
      if (storage) storage.set('enabled', next)
      if (!next) flush()
      haptic('tap')
      host.notify({ kind: 'info', message: `sonus-read ${next ? 'on' : 'off'}` })
    },
    children: on ? 'leer: on' : 'leer: off'
  })
}

export default {
  id: ID,
  name: 'Sonus Read',
  register(ctx) {
    storage = ctx.storage
    $enabled.set(Boolean(storage?.get('enabled', false)))
    ctx.onDispose(host.onEvent('message.delta', onDelta))
    ctx.onDispose(host.onEvent('message.complete', flush))
    ctx.onDispose(host.onEvent('message.interim', flush))
    ctx.register({
      id: 'chip',
      area: 'statusBar.right',
      order: 160,
      render: () => jsx(Chip, {})
    })
  }
}
