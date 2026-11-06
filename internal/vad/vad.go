// Package vad wraps Silero VAD from sherpa-onnx.
package vad

import (
	"fmt"
	"os"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"
)

type VAD struct {
	v *sherpa.VoiceActivityDetector
}

func Open(model string) (*VAD, error) {
	if _, err := os.Stat(model); err != nil {
		return nil, fmt.Errorf("VAD no encontrado: %s", model)
	}
	cfg := sherpa.VadModelConfig{}
	cfg.SileroVad.Model = model
	cfg.SileroVad.Threshold = 0.5
	cfg.SileroVad.MinSilenceDuration = 0.5
	cfg.SileroVad.MinSpeechDuration = 0.25
	cfg.SileroVad.WindowSize = 512
	cfg.SileroVad.MaxSpeechDuration = 20
	cfg.SampleRate = 16000
	cfg.NumThreads = 1
	cfg.Provider = "cpu"
	v := sherpa.NewVoiceActivityDetector(&cfg, 30)
	if v == nil {
		return nil, fmt.Errorf("no se pudo cargar Silero VAD: %s", model)
	}
	return &VAD{v: v}, nil
}

func (v *VAD) Speech(samples []float32) bool {
	if v == nil || v.v == nil || len(samples) == 0 {
		return false
	}
	v.v.AcceptWaveform(samples)
	return v.v.IsSpeech()
}

func (v *VAD) Close() {
	if v != nil && v.v != nil {
		sherpa.DeleteVoiceActivityDetector(v.v)
		v.v = nil
	}
}
