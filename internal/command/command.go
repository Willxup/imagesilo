package command

import (
	"errors"
	"fmt"
)

var errUsage = errors.New("usage: imagesilo <serve|healthcheck|admin create --email ADDRESS [--password-stdin]>")

// Run parses the deliberately small command surface and dispatches a command.
func Run(args []string) error {
	if len(args) == 0 {
		return errUsage
	}

	switch args[0] {
	case "serve":
		return serve()
	case "healthcheck":
		return healthcheck()
	case "admin":
		return admin(args[1:])
	case "help", "-h", "--help":
		return fmt.Errorf("%w", errUsage)
	default:
		return fmt.Errorf("unknown command %q: %w", args[0], errUsage)
	}
}
