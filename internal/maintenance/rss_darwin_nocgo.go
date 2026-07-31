//go:build darwin && !cgo

package maintenance

func currentRSSBytes() uint64 {
	return 0
}
