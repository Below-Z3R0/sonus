# Sonus Read — Hermes desktop plugin

Lee las respuestas de Hermes en voz alta con Vox, por frase, mientras el texto llega en
`message.delta`.

## Requisitos

```bash
voxctl service load kokoro-82m-onnx
```

Vox tiene que estar en `127.0.0.1:9102`; `misakid` también. El plugin no carga modelos.

## Instalación

```bash
mkdir -p ~/.hermes/desktop-plugins/sonus-read
cp plugin.js ~/.hermes/desktop-plugins/sonus-read/
```

Después, en Hermes Desktop: `⌘K` → **Reload desktop plugins**. Aparece el chip `leer: off` en la
status bar; al activarlo consume los deltas por frase y manda cada oración a
`POST /v1/speech/stream`.

## Protocolo

```text
message.delta (Hermes)
  → acumula texto
  → corta en . ! ? …
  → POST /v1/speech/stream
  → PCM16 24 kHz
  → Web Audio API
```

No espera a que Hermes termine la respuesta: empieza con la primera oración.

## Límites

- Depende del shape del evento `message.delta`; si Hermes cambia el payload, ajustar
  `onDelta()`.
- No hay VAD ni interrupción por voz: el toggle corta el encolado, no el audio ya programado.
- El navegador/Electron necesita CORS en Vox; el endpoint de stream lo habilita.
