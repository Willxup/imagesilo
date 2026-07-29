//go:build cgo && vips

package processor

import "github.com/davidbyttow/govips/v2/vips"

// VIPSVersion exists as an early compile-time compatibility probe. The real
// processor is introduced in phase 3 after the two-architecture smoke test.
func VIPSVersion() string {
	return vips.Version
}
