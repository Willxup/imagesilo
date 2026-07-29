//go:build !linux && !darwin

package maintenance

func currentRSSBytes() uint64 {
	return 0
}
