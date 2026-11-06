package runtime

import "testing"

func TestLoadUnloadLifecycle(t *testing.T) {
	loaded := ""
	mgr := New("vox", "1.0", "kokoro-82m-onnx",
		func(id string) error { loaded = id; return nil },
		func() { loaded = "" },
	)
	if mgr.Status().State != "not_loaded" {
		t.Fatal("new manager must be not_loaded")
	}
	if err := mgr.Load("kokoro-82m-onnx"); err != nil {
		t.Fatal(err)
	}
	st := mgr.Status()
	if !st.Loaded || st.State != "ready" || loaded != "kokoro-82m-onnx" {
		t.Fatalf("status = %+v loaded=%q", st, loaded)
	}
	mgr.Unload()
	st = mgr.Status()
	if st.Loaded || st.State != "not_loaded" || loaded != "" {
		t.Fatalf("after unload = %+v loaded=%q", st, loaded)
	}
}
