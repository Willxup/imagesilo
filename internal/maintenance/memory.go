package maintenance

import "runtime"

type RuntimeSnapshot struct {
	HeapAllocBytes uint64
	HeapSysBytes   uint64
	Goroutines     int
}

func CaptureRuntime() RuntimeSnapshot {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return RuntimeSnapshot{
		HeapAllocBytes: stats.HeapAlloc,
		HeapSysBytes:   stats.HeapSys,
		Goroutines:     runtime.NumGoroutine(),
	}
}
