"""misakid: Kokoro-compatible phonemizer service.

The service owns the text frontend that Kokoro needs: normalization, G2P and
the phoneme string that voxd converts to PhonemeCodepoints. Spanish goes
through misaki.espeak (espeak-ng under the hood); English stays on the same
engine until the native misaki.en model is installed.
"""

from __future__ import annotations

import threading

import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from misaki import espeak

app = FastAPI(title="misakid", version="1.0.0")

_aliases = {
    "es": "es",
    "es-es": "es",
    "es-mx": "es",
    "en": "en-us",
    "en-us": "en-us",
    "en-gb": "en-gb",
    "fr": "fr-fr",
    "fr-fr": "fr-fr",
    "it": "it",
    "pt": "pt-br",
    "pt-br": "pt-br",
    "hi": "hi",
}
_cache: dict[str, espeak.EspeakG2P] = {}
_lock = threading.Lock()


class Request(BaseModel):
    text: str
    lang: str = "es"


def engine(lang: str) -> espeak.EspeakG2P:
    code = _aliases.get(lang.lower())
    if not code:
        raise HTTPException(status_code=400, detail=f"unsupported language: {lang}")
    with _lock:
        if code not in _cache:
            _cache[code] = espeak.EspeakG2P(language=code)
        return _cache[code]


@app.get("/healthz")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/v1/phonemize")
def phonemize(req: Request) -> dict:
    text = req.text.strip()
    if not text:
        raise HTTPException(status_code=400, detail="empty text")
    if len(text) > 2000:
        raise HTTPException(status_code=413, detail="text too long")
    g2p = engine(req.lang)
    with _lock:
        phonemes, _tokens = g2p(text)
    return {
        "lang": req.lang,
        "phonemes": phonemes,
        "codepoints": [ord(ch) for ch in phonemes],
    }


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=9200)
