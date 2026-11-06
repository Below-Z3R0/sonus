//go:build linux && cgo

package memtrim

/*
#include <malloc.h>
*/
import "C"

// Trim asks glibc to return free arenas to the OS after a model unload.
func Trim() {
	C.malloc_trim(0)
}
