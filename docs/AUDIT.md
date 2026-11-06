# Auditoría de Sonus

Estado después de conectar Kokoro, Whisper, jobs, MCP y configuración.

## Lo que ya está cubierto

| Área | Vox | Auris |
|---|---|---|
| Catálogo separado | sí | sí |
| Descarga con progreso, SHA-256 y disco | sí | sí |
| RAM guard, preload, unload, `malloc_trim` | sí | sí |
| Un modelo residente; load/unload sin reiniciar | sí | sí |
| Texto largo / audio largo | troceo por caracteres | ventanas de 30 s |
| Idioma | detección por fragmento | Whisper detecta |
| Jobs en memoria | sí (journal en disco) | sí (journal en disco) |
| SSE | `/v1/jobs/{id}/events` | igual |
| Streaming TTS | PCM16 por frase | no aplica |
| Streaming ASR | `streamd` WebSocket + Kroko | no aplica |
| Micrófono host | `streamctl listen` + gate de energía | no aplica |
| Test WebSocket | fixture WAV gateado por env | no aplica |
| MCP | `vox_*` | `auris_*` |
| Config get/set/reset | sí | sí |
| Doctor y métricas | sí | sí |
| Formatos de entrada | texto | WAV, MP3/OGG vía ffmpeg en CLI |

## Auditoría de llamadas y estados

Revisado en `cmd/`, `internal/` y los tests:

- **No hay doble carga de modelo.** `runtime.Manager.Load` devuelve temprano si el modelo ya está
  `ready`; `Load` sólo reemplaza el residente cuando cambia el id. Los jobs no cargan modelos: usan
  el motor ya residente.
- **No hay doble lectura de audio.** `aurisd` guarda el multipart una vez, `sherpa.ReadWave` lee una
  vez y el motor itera los chunks en memoria.
- **No hay doble descarga.** `modeldl.Install` abre una sola conexión, escribe un temporal, calcula
  SHA-256 en el mismo stream y extrae una vez.
- **No hay doble llamada al phonemizer por texto.** `Engine.Speak` llama una vez por fragmento; el
  fragmento es la unidad que Kokoro necesita. No se vuelve a fonemizar el texto completo.
- **Estados separados.** Artefacto (`missing/downloading/ready/failed`) y runtime
  (`not_loaded/loading/ready/error`) son ejes distintos. `models use` sólo escribe el default;
  `service load` es el que aplica el residente.
- **Duplicación de código, no de I/O.** `aurisctl` tiene dos funciones de upload (sync y async) con
  el mismo armado multipart; se puede extraer un helper. `voxd`/`aurisd` hacen `catalog.Get` en el
  `main` y otra vez en `Engine.Load`; es un lookup en memoria, no una carga.
- **MCP no ejecuta CLIs.** Las tools llaman HTTP a los daemons; no hay un segundo proceso cargando
  modelos ni una segunda copia de config.

## Gaps y mejoras recomendadas

### Prioridad alta

1. **VAD y solapamiento en ASR.** `streamctl listen` tiene gate de energía en el host y `streamd`
   puede usar Silero VAD en el servidor (`streamctl vad install`). Auris offline sigue cortando cada
   30 s sin overlap; para archivos largos conviene VAD + 2–3 s de overlap con deduplicación.
2. **Streaming ASR: calidad.** `streamd` ya acepta PCM16 por WebSocket, usa Silero VAD opcional y
   devuelve parciales con Kroko. Falta ajustar endpoint/overlap y medir WER en español; el corte
   final lo decide `EnableEndpoint`.
3. **Concurrencia del motor.** `Engine.Speak` y `Engine.Transcribe` serializan todo con un mutex:
   evita OOM, pero un job largo bloquea a los demás. Medir y decidir pool o límite de cola.

### Prioridad media

5. **Formato en MCP.** `auris_transcribe` del MCP sólo acepta WAV; el CLI ya convierte MP3/OGG con
   ffmpeg. Mover la conversión a un paquete compartido para que ambos caminos sean iguales.
6. **Métricas de calidad.** `/v1/metrics` es JSON de contadores. Agregar RTF (tiempo de audio /
   tiempo real), latencia p50/p95, tamaño de cola y formato Prometheus si se va a monitorear.
7. **VAD/energía en Vox.** El troceo por caracteres no detecta pausas naturales; para audiolibros o
   respuestas largas conviene pausa por frase y velocidad por voz.
8. **`misaki.en` nativo pesa.** `misaki[en]` arrastra torch/spacy; la imagen de `misakid` crece por
   encima de 1 GB. Si el tamaño importa, usar el fallback espeak en la imagen mínima y dejar el
   módulo nativo como imagen `misaki:full`.
9. **Provider real por variante.** Falta validar en hardware AMD/Intel que el provider cargado sea
   el esperado; el `doctor` actual informa estado del modelo, no el EP efectivo de sherpa.

### Prioridad baja

10. **Tests de motor.** No hay tests que carguen Kokoro/Whisper en CI por tamaño; conviene un test
    live gateado por variable de entorno y un fixture corto.
11. **Límites de body y timeouts** están en HTTP, pero conviene un límite explícito de duración de
    audio y de cantidad de texto por request.
12. **Observabilidad de jobs**: quién los creó, duración por fase, último error; hoy sólo estado y
    resultado.

### Modelos locales para ASR streaming

"Online" en sherpa-onnx significa **streaming**, no nube. Todos corren en la máquina con ONNX Runtime:

| Modelo | Licencia | Tamaño | Notas |
|---|---|---|---|
| Kroko ES 64L / 128L (`hudaiapa88/sherpa-stt-onnx`, `es/kroko_*`) | Apache-2.0 | 147 MB int8 | Zipformer2 transducer, 3 archivos + tokens; recomendado para probar |
| `csukuangfj/sherpa-onnx-streaming-zipformer-es-kroko-2025-08-06` | sin licencia explícita en la card | 155 MB encoder | Mismo origen Kroko; verificar licencia antes de publicar |
| `bookbot/sherpa-onnx-zipformer-streaming-robust-es-v0` | Apache-2.0 | ONNX | Salida en fonemas IPA; necesita texto/lexicón, no es plug-and-play |
| `bookbot/sherpa-onnx-ort-zipformer-streaming-robust-es-v1` | CC-BY-NC-SA-3.0 | ORT | No comercial; `.ort`, no el formato estándar |
| Vosk español | Apache-2.0 | variable | Streaming real, pero otro runtime (no sherpa/ORT) |
| Whisper-Streaming + VAD | MIT (Whisper) | el Whisper actual | ~3–5 s de latencia; reutiliza el modelo sin agregar otro |

La opción de menor fricción para Sonus es **Kroko ES 64L/128L** con `OnlineRecognizer`; la de menor
modelo nuevo es **Whisper-Streaming**.

Medición inicial con el fixture `testdata/stream-es.wav`:

- `kroko-es-64l`: devuelve `Hola, esto es una` (parcial/final); es el default.
- `kroko-es-128l`: carga, pero no devuelve texto. El encoder 128L espera `x=[N,269,80]` y metadata
  `T=269`/`decode_chunk_len=256`; el 64L espera `[N,141,80]`. La documentación de audio.cpp confirma
  los dos layouts públicos (`141/128` y `269/256`). El runtime Go actual no adapta el chunk a 269,
  así que el 128L no es usable con esta versión de sherpa; no se cambia el default.

## Referencias

- Whisper long audio: chunking 20–30 s, overlap 2–3 s, VAD y deduplicación.
- Kokoro: frontend oficial `misaki`; español vía `misaki.espeak` (espeak-ng), inglés vía
  `misaki.en`; el modelo acepta `PhonemeCodepoints`.
- MCP: Streamable HTTP con un proceso compartido; jobs para comandos largos y salida paginada.
