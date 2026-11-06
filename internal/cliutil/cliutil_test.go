package cliutil

import (
	"reflect"
	"testing"
)

func TestSplitFlagsAfterPositional(t *testing.T) {
	flags, positional := SplitFlags(
		[]string{"reunion.mp4", "--out", "dir", "--voice", "ef_dora"},
		map[string]bool{"--out": true, "--voice": true},
	)
	if !reflect.DeepEqual(positional, []string{"reunion.mp4"}) {
		t.Fatalf("positional = %v", positional)
	}
	want := []string{"--out", "dir", "--voice", "ef_dora"}
	if !reflect.DeepEqual(flags, want) {
		t.Fatalf("flags = %v, want %v", flags, want)
	}
}

func TestSplitFlagsBoolean(t *testing.T) {
	flags, positional := SplitFlags([]string{"file", "--json"}, nil)
	if !reflect.DeepEqual(positional, []string{"file"}) || !reflect.DeepEqual(flags, []string{"--json"}) {
		t.Fatalf("flags=%v positional=%v", flags, positional)
	}
}
