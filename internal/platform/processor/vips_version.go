//go:build cgo && vips

package processor

import (
	"sync"

	"github.com/davidbyttow/govips/v2/vips"
)

var (
	vipsStartupOnce sync.Once
	vipsStartupErr  error
)

func VIPSVersion() string {
	return vips.Version
}

func Startup() error {
	vipsStartupOnce.Do(func() {
		vips.LoggingSettings(nil, vips.LogLevelWarning)
		vipsStartupErr = vips.Startup(&vips.Config{
			ConcurrencyLevel: 1,
			MaxCacheFiles:    0,
			MaxCacheMem:      0,
			MaxCacheSize:     0,
		})
	})
	return vipsStartupErr
}

func Shutdown() {
	vips.Shutdown()
}
