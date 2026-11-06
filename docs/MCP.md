# Sonus MCP

`sonus-mcp` expone los daemons residentes, no los CLIs. No carga modelos, no abre archivos y no
duplica el pipeline: cada tool hace HTTP contra `voxd` o `aurisd`.

```text
Hermes / cliente MCP
        ↓ Streamable HTTP :9201/mcp
sonus-mcp
   ├── http://vox:9102
   └── http://auris:9101
```

## Arranque

```bash
sonus-mcp --addr 127.0.0.1:9201 \
  --vox http://127.0.0.1:9102 \
  --auris http://127.0.0.1:9101
```

En Compose, el servicio `mcp` ya apunta a `vox:9102` y `auris:9101`.

## Configuración en Hermes

```yaml
mcp_servers:
  sonus:
    url: http://127.0.0.1:9201/mcp
    enabled: true
```

## Tools

| Tool | Función |
|---|---|
| `vox_status` / `auris_status` | modelo residente y estado |
| `vox_doctor` / `auris_doctor` | disco, RAM, modelo, salud de misakid |
| `vox_models` / `auris_models` | catálogo e instalación |
| `vox_speak` | encola TTS y devuelve `job_id` |
| `auris_transcribe` | transcribe un WAV del host y devuelve texto |
| `vox_jobs` / `auris_jobs` | lista de jobs |
| `vox_job` / `auris_job` | estado/resultado de un job |
| `vox_load` / `auris_load` / `stream_load` | carga un modelo |
| `vox_unload` / `auris_unload` / `stream_unload` | libera el residente |

`--read-only` no registra las tools de escritura.

## Jobs

`vox_speak` y `auris_transcribe` son operaciones cortas para el agente, pero también existen
`/v1/speech/jobs` y `/v1/transcribe/jobs` en los daemons. El MCP devuelve el `job_id`; el agente
consulta `vox_job` o `auris_job` hasta `done` o `failed`.

Los jobs viven en memoria, con retención de los últimos 50. No sobreviven un reinicio; para trabajo
durable, el job debe persistirse fuera del proceso.

## Seguridad

- Bind a loopback por defecto.
- Si se expone fuera de loopback, usar Tailscale o token; no publicar `0.0.0.0` sin auth.
- `auris_transcribe` lee un path del host: el proceso MCP debe tener acceso a ese path y el path no
  debe venir de un usuario no confiable.
