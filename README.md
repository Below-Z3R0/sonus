# sonus

Stack de voz local en Go sobre ONNX Runtime. Dos servicios, una imagen:

- **Auris**: audio → texto (ASR).
- **Vox**: texto → audio (TTS).

Comparte las bases `b3l0w-ort:*` con Argos. Los modelos viven en un volumen, no en la imagen.

## Estado

Scaffold funcional: servidores HTTP, CLI, catálogo ONNX y compose. Los motores de inferencia
(Whisper/Kokoro) todavía no están conectados; los endpoints devuelven `501` hasta la fase 2.

## Desarrollo

```bash
go test ./...
go build ./cmd/...
```

Imagen local:

```bash
docker build --build-arg BASE_IMAGE=b3l0w-ort:cpu-test -t sonus:test .
docker run --rm sonus:test aurisctl version
```

## Compose

```bash
cp .env.example .env
docker compose up -d
```

Servicios:

| Servicio | URL | Función |
|---|---|---|
| auris | http://127.0.0.1:9101 | transcripción |
| vox | http://127.0.0.1:9102 | síntesis de voz |

## Modelos

Solo ONNX:

- ASR: Whisper large-v3-turbo / small.
- TTS: Kokoro-82M / Piper Spanish.
- VAD: Silero.

`aurisctl models` y `voxctl models` muestran el catálogo; `models use` cambiará el default cuando
el motor esté conectado.

## Plan

Ver [`docs/PLAN.md`](docs/PLAN.md).
