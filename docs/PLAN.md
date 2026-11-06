# Plan de sonus

## Fase 1 — scaffold

- [x] Repo Go, catálogo ONNX, servidores HTTP, CLIs, compose.
- [x] Imagen sobre `b3l0w-ort:*`.
- [x] Workflow matrix por variante.
- [ ] `.env.example`.

## Fase 2 — Vox TTS

- [ ] Integrar Kokoro ONNX (o Piper como fallback).
- [ ] `POST /v1/speech` con texto, voz y formato.
- [ ] `voxctl speak`, `--file`, `--stdin`, `--out`, `--play`.
- [ ] Voces españolas y presets.
- [ ] MCP `vox_speak`.

## Fase 3 — Auto-read

- [ ] Plugin de Hermes `vox-autoread`.
- [ ] Comando “leer última respuesta”.
- [ ] Toggle opcional; filtros de código y mensajes vacíos.
- [ ] `vox-stop`.

## Fase 4 — Auris ASR

- [ ] Whisper large-v3-turbo ONNX.
- [ ] `POST /v1/transcribe` multipart.
- [ ] Jobs + SSE para audios largos.
- [ ] `aurisctl transcribe`, `--watch`, `--out`, formatos.
- [ ] MCP `auris_transcribe`.

## Fase 5 — Micrófono

- [ ] `aurisctl devices`.
- [ ] `aurisctl listen` push-to-talk.
- [ ] `aurisctl listen --stream` con VAD y WebSocket.
- [ ] Captura en host (`pw-record`/`parec`/`ffmpeg`); el contenedor solo recibe PCM.

## Fase 6 — Noctalia

- [ ] Botón de micrófono.
- [ ] Selector de voz.
- [ ] Leer notificaciones.

## Reglas

- Solo modelos con export ONNX.
- Modelos fuera de la imagen.
- Sin diarización.
- Dos contenedores en el mismo compose.
- Base ORT compartida con Argos.
