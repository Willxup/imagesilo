package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"os"
	"sort"
	"sync"
	"time"
)

type sessionResponse struct {
	CSRFToken string `json:"csrfToken"`
}

type result struct {
	Concurrency       int     `json:"concurrency"`
	Requests          int     `json:"requests"`
	Successes         int     `json:"successes"`
	BusyResponses     int     `json:"busyResponses"`
	P95Milliseconds   float64 `json:"p95Milliseconds"`
	ThroughputPerSec  float64 `json:"throughputPerSecond"`
	SourceFormat      string  `json:"sourceFormat"`
	SourceWidth       int     `json:"sourceWidth"`
	SourceHeight      int     `json:"sourceHeight"`
	SourcePixels      int     `json:"sourcePixels"`
	SourceFixtureSize int     `json:"sourceFixtureBytes"`
	StoredBytesTotal  int64   `json:"storedBytesTotal"`
	StoredBytesMean   float64 `json:"storedBytesMean"`
}

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:18086", "ImageSilo base URL")
	concurrency := flag.Int("concurrency", 1, "parallel upload count")
	requests := flag.Int("requests", 16, "total upload count")
	width := flag.Int("width", 5000, "source fixture width")
	height := flag.Int("height", 3200, "source fixture height")
	conversionEnabled := flag.Bool("conversion-enabled", true, "enable PNG to WebP conversion")
	flag.Parse()
	if *concurrency < 1 || *requests < 1 || *width < 1 || *height < 1 {
		fatal("concurrency, requests, width, and height must be positive")
	}
	password := os.Getenv("IMAGESILO_BENCH_PASSWORD")
	if password == "" {
		fatal("IMAGESILO_BENCH_PASSWORD is required")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		fatal(err.Error())
	}
	client := &http.Client{Jar: jar, Timeout: 2 * time.Minute}
	csrfToken := login(client, *baseURL, password)
	updateProcessing(client, *baseURL, csrfToken, *conversionEnabled)
	pngBytes := benchmarkPNG(*width, *height)
	body, contentType := multipartBody(pngBytes)

	jobs := make(chan struct{})
	durations := make(chan time.Duration, *requests)
	statuses := make(chan int, *requests)
	storedSizes := make(chan int64, *requests)
	var workers sync.WaitGroup
	for worker := 0; worker < *concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range jobs {
				started := time.Now()
				request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, *baseURL+"/api/v1/images", bytes.NewReader(body))
				if err != nil {
					fatal(err.Error())
				}
				request.Header.Set("Content-Type", contentType)
				request.Header.Set("X-CSRF-Token", csrfToken)
				response, err := client.Do(request)
				if err != nil {
					fatal(err.Error())
				}
				responseBody, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					fatal(err.Error())
				}
				storedSize := int64(0)
				if response.StatusCode == http.StatusCreated {
					var uploaded struct {
						StoredSize int64 `json:"storedSize"`
					}
					if err := json.Unmarshal(responseBody, &uploaded); err != nil || uploaded.StoredSize <= 0 {
						fatal("upload response did not include storedSize")
					}
					storedSize = uploaded.StoredSize
				}
				durations <- time.Since(started)
				statuses <- response.StatusCode
				storedSizes <- storedSize
			}
		}()
	}
	started := time.Now()
	for request := 0; request < *requests; request++ {
		jobs <- struct{}{}
	}
	close(jobs)
	workers.Wait()
	close(durations)
	close(statuses)
	close(storedSizes)
	elapsed := time.Since(started)

	values := make([]time.Duration, 0, *requests)
	for duration := range durations {
		values = append(values, duration)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	successes := 0
	busy := 0
	for status := range statuses {
		switch status {
		case http.StatusCreated:
			successes++
		case http.StatusServiceUnavailable:
			busy++
		default:
			fatal(fmt.Sprintf("unexpected upload status %d", status))
		}
	}
	storedBytesTotal := int64(0)
	for storedSize := range storedSizes {
		storedBytesTotal += storedSize
	}
	p95Index := int(float64(len(values)-1) * 0.95)
	output := result{
		Concurrency: *concurrency, Requests: *requests, Successes: successes, BusyResponses: busy,
		P95Milliseconds:  float64(values[p95Index].Microseconds()) / 1000,
		ThroughputPerSec: float64(successes) / elapsed.Seconds(),
		SourceFormat:     "png", SourceWidth: *width, SourceHeight: *height,
		SourcePixels: *width * *height, SourceFixtureSize: len(pngBytes),
		StoredBytesTotal: storedBytesTotal,
	}
	if successes > 0 {
		output.StoredBytesMean = float64(storedBytesTotal) / float64(successes)
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fatal(err.Error())
	}
	if successes != *requests || busy != 0 {
		os.Exit(1)
	}
}

func login(client *http.Client, baseURL, password string) string {
	body, _ := json.Marshal(map[string]string{"email": "admin@example.com", "password": password})
	response, err := client.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		fatal(err.Error())
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fatal(fmt.Sprintf("login status %d", response.StatusCode))
	}
	var session sessionResponse
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil || session.CSRFToken == "" {
		fatal("login response did not include a CSRF token")
	}
	return session.CSRFToken
}

func updateProcessing(client *http.Client, baseURL, csrfToken string, conversionEnabled bool) {
	body, err := json.Marshal(map[string]any{
		"compressionEnabled": false, "jpegQuality": 85, "webpQuality": 82, "pngCompressionLevel": 6,
		"conversionEnabled": conversionEnabled, "conversionWebpQuality": 82, "conversionWebpLossless": false,
	})
	if err != nil {
		fatal(err.Error())
	}
	request, err := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/settings/processing", bytes.NewReader(body))
	if err != nil {
		fatal(err.Error())
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", csrfToken)
	response, err := client.Do(request)
	if err != nil {
		fatal(err.Error())
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fatal(fmt.Sprintf("processing settings status %d", response.StatusCode))
	}
}

func multipartBody(data []byte) ([]byte, string) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "benchmark.png")
	if err != nil {
		fatal(err.Error())
	}
	if _, err := part.Write(data); err != nil {
		fatal(err.Error())
	}
	if err := writer.Close(); err != nil {
		fatal(err.Error())
	}
	return body.Bytes(), writer.FormDataContentType()
}

func benchmarkPNG(width, height int) []byte {
	value := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value.SetRGBA(x, y, color.RGBA{
				R: uint8((x*13 + y*3) % 256), G: uint8((x*5 + y*11) % 256),
				B: uint8((x + y*7) % 256), A: 255,
			})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, value); err != nil {
		fatal(err.Error())
	}
	return buffer.Bytes()
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
