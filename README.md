# sonus

Stack de voz local en Go sobre ONNX Runtime. Dos servicios, una imagen:

- **Auris**: audio → texto (ASR).
- **Vox**: texto → audio (TTS).

Comparte las bases `b3l0w-ort:*` con Argos. Los modelos viven en un volumen, no en la imagen.

## Estado

Scaffold funcional con Vox y Auris conectados: `voxd` carga Kokoro ONNX, usa `misakid` como
frontend de texto y devuelve WAV real; `aurisd` carga Whisper ONNX y transcribe WAV a texto.

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
| misakid | interno :9200 | frontend de texto para Kokoro |

## Sintetizar voz

```bash
voxctl speak "Hola, esto es una prueba." --out hola.wav
voxctl speak "Hello, this is a test." --out hello.wav
voxctl speak "Hola" --lang en --out forzado.wav
voxctl speak "texto largo" --file texto.txt --out texto.wav
voxctl speak "texto largo" --async
voxctl jobs wait <job_id>
```

`--lang` es `auto` por defecto: `voxd` detecta español o inglés en el texto y elige la voz del preset.
`--voice` permite forzar una voz concreta. `--async` devuelve un job y el WAV queda en
`~/.local/share/sonus/output/tts-*.wav` (o el directorio configurado).

`voxd` usa Kokoro; `misakid` convierte el texto a fonemas con el frontend oficial de Kokoro
(misaki; para español, `misaki.espeak` con espeak-ng; para inglés, `misaki.en`). El texto se divide
en fragmentos de `max_chars` (400 por defecto), se detecta el idioma de cada fragmento y los WAV se
concatenan en uno solo a 24 kHz, 16 bit, mono.

Voces Kokoro incluidas en el preset: `em_santa` (es, sid 53), `ef_dora` (es, sid 28),
`em_alex` (es, sid 29), `af_heart` (en-US, sid 3), `am_puck` (en-US, sid 18).

## Transcribir audio

```bash
aurisctl models install whisper-turbo-onnx
aurisctl service load whisper-turbo-onnx
aurisctl transcribe audio.wav
aurisctl transcribe audio.wav --out transcripciones
aurisctl transcribe audio.mp3
aurisctl transcribe audio.wav --async
aurisctl jobs wait <job_id>
```

La entrada puede ser WAV, MP3 u OGG: `aurisctl` convierte a WAV mono 16 kHz con ffmpeg antes de
subirlo. El servidor parte audios de más de 30 segundos en bloques y devuelve `{text, lang}`.

## Diagnóstico y métricas

```bash
curl -sS http://127.0.0.1:9102/v1/doctor
curl -sS http://127.0.0.1:9102/v1/metrics
curl -sS http://127.0.0.1:9101/v1/doctor
curl -sS http://127.0.0.1:9101/v1/metrics
```

`doctor` muestra estado del modelo, espacio libre, RAM disponible y, en Vox, salud de `misakid`.
`metrics` cuenta requests, errores y duración acumulada.

## Jobs y MCP

Las operaciones largas se pueden encolar:

```bash
voxctl speak "texto largo" --async
aurisctl transcribe audio.wav --async
voxctl jobs list
voxctl jobs status <id>
voxctl jobs wait <id>
```

`sonus-mcp` expone ambos servicios por MCP Streamable HTTP:

```bash
sonus-mcp --addr 127.0.0.1:9201
```

Tools: `vox_speak`, `vox_status`, `vox_doctor`, `vox_models`, `vox_jobs`, `vox_job`, `vox_load`,
`vox_unload`, y los equivalentes `auris_*`. `--read-only` oculta las tools de escritura.
Configuración y contrato completo en [`docs/MCP.md`](docs/MCP.md).

## Config

```bash
voxctl config get defaults.tts
voxctl config set defaults.tts kokoro-82m-onnx
voxctl config reset defaults.tts
aurisctl config get defaults.asr
```

Claves: `variant`, `paths.*`, `defaults.asr`, `defaults.tts`, `services.phonemizer`.

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
kokoro-82m-onnx  balanced  *        ready     5 voices
piper-es-onnx    fast               missing   2 voices
```

- `missing`: no está en el volumen.
- `downloading 42%`: descarga en curso; el porcentaje queda en `.state.json` si se corta.
- `ready`: instalado y verificado.
- `failed`: error; `models install --force` reintenta.

`models install` descarga con progreso (`%` y MB), verifica **SHA-256** cuando el catálogo lo tiene,
extrae el `.tar.bz2` y escribe `.installed.json`. Antes de descargar chequea espacio libre en el
volumen. `models remove` borra el modelo. `service load` rechaza un modelo que no esté `ready` y
verifica RAM disponible contra el presupuesto del modelo.

`voxd --preload` y `aurisd --preload` cargan el default al arrancar. `unload` cierra el modelo,
libera memoria de Go y llama `malloc_trim` para devolver arenas de glibc al SO.

## Modelos

Catálogos separados, como Argos:

- **Auris** (`aurisctl models`): solo ASR.
  ```text
  whisper-turbo-onnx   balanced  *
  whisper-small-onnx   fast
  ```
- **Vox** (`voxctl models`): solo TTS y sus voces.
  ```text
  kokoro-82m-onnx   balanced  *  5 voces
  piper-es-onnx     fast          2 voces
  ```

`models use <id>` cambia el default y lo guarda en `~/.config/sonus/config.yaml`; el servicio lo
carga al reiniciar. Los modelos no están en la imagen: van al volumen y se gestionan por CLI.

Kokoro es el default de Vox; incluye voces españolas (`ef_dora`, `em_alex`, `em_santa`). Piper queda
como fallback liviano.

## Plan

Ver [`docs/PLAN.md`](docs/PLAN.md).
