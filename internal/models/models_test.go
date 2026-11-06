package models

import "testing"

func TestCatalogTasks(t *testing.T) {
	if len(List(TaskASR)) == 0 || len(List(TaskTTS)) == 0 || len(List(TaskVAD)) == 0 {
		t.Fatal("every task needs at least one ONNX model")
	}
	m, ok := Get("whisper-turbo-onnx")
	if !ok || m.Task != TaskASR {
		t.Fatalf("whisper entry: %+v %v", m, ok)
	}
}
