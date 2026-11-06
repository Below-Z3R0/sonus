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

## Ciclo de vida del modelo

Cada servicio mantiene **un solo modelo residente**:

```bash
voxctl service status
voxctl service load kokoro-82m-onnx
voxctl service unload
```

`unload` cierra el modelo y llama `debug.FreeOSMemory`; el contenedor, el HTTP y el MCP (cuando esté)
siguen vivos. No hace falta apagar Docker para liberar RAM.

`models use <id>` solo cambia el default y exige que el modelo esté instalado. Para aplicarlo:

```bash
voxctl models install kokoro-82m-onnx
voxctl models use kokoro-82m-onnx
voxctl service load kokoro-82m-onnx
```

## Estados de descarga

```bash
voxctl models list
ID               TIER      DEFAULT  STATE     EXTRA
kokoro-82m-onnx  balanced  *        ready     4 voices
piper-es-onnx    fast               missing   2 voices
```

- `missing`: no está en el volumen.
- `downloading 42%`: descarga en curso; el porcentaje queda en `.state.json` si se corta.
- `ready`: instalado y verificado.
- `failed`: error; `models install --force` reintenta.

`models install` descarga con progreso (`%` y MB), extrae el `.tar.bz2` y escribe `.installed.json`.
`models remove` borra el modelo del volumen. `service load` rechaza un modelo que no esté `ready`.

## Modelos

Catálogos separados, como Argos:

- **Auris** (`aurisctl models`): solo ASR.
  ```text
  whisper-turbo-onnx   balanced  *
  whisper-small-onnx   fast
  ```
- **Vox** (`voxctl models`): solo TTS y sus voces.
  ```text
  kokoro-82m-onnx   balanced  *  4 voces
  piper-es-onnx     fast          2 voces
  ```

`models use <id>` cambia el default y lo guarda en `~/.config/sonus/config.yaml`; el servicio lo
carga al reiniciar. Los modelos no están en la imagen: van al volumen y se gestionan por CLI.

Kokoro es el default de Vox; incluye voces españolas (`ef_dora`, `em_alex`). Piper queda como
fallback liviano.

## Plan

Ver [`docs/PLAN.md`](docs/PLAN.md).
