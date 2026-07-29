package command

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func healthcheck() error {
	url := os.Getenv("IMAGESILO_HEALTHCHECK_URL")
	if url == "" {
		url = "http://127.0.0.1:8080/healthz"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health request returned %s", response.Status)
	}
	return nil
}
