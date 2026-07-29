//go:build cgo && vips && linux

package processor

/*
#include <malloc.h>
*/
import "C"

// TrimMemory returns native image-processing allocations that glibc would
// otherwise retain after large uploads. The processing gate bounds active
// work; trimming bounds the idle high-water mark observed after that work.
func TrimMemory() {
	C.malloc_trim(0)
}
