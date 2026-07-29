package webui

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var embedded embed.FS

type UI struct {
	filesystem fs.FS
	index      []byte
}

func New() (*UI, error) {
	filesystem, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded web UI: %w", err)
	}
	index, err := fs.ReadFile(filesystem, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read embedded web UI index: %w", err)
	}
	return &UI{filesystem: filesystem, index: index}, nil
}

func (u *UI) Assets() http.Handler {
	return http.FileServer(http.FS(u.filesystem))
}

func (u *UI) ServeIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(u.index)
}
