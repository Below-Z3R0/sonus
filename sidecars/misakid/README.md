# misakid

Frontend de texto para Kokoro. Recibe texto y devuelve fonemas y codepoints.

```sh
curl -sS -X POST http://127.0.0.1:9200/v1/phonemize \
  -H 'content-type: application/json' \
  -d '{"text":"Hola, esto es una prueba.","lang":"es"}'
```

```json
{"lang":"es","phonemes":"ˈola, ˈesto ˈes ˈuna pɾuˈeβa.","codepoints":[712,111,108,97,...]}
```

- Español: `misaki.espeak.EspeakG2P` (espeak-ng debajo).
- Inglés: `misaki.en` nativo para en-US/en-GB, con fallback de espeak.
- El servicio no sintetiza audio; solo produce el contrato de fonemas que `voxd` pasa a
  `GenerationConfig.PhonemeCodepoints`.
- `GET /healthz` para el healthcheck de compose.
