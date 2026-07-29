//go:build !vips || !cgo

package processor

func Startup() error {
	return nil
}

func Shutdown() {}

func VIPSVersion() string {
	return "disabled"
}
