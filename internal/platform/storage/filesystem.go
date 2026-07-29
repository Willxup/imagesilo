package storage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Filesystem struct {
	imagesDirectory    string
	temporaryDirectory string
}

func NewFilesystem(dataDirectory string) *Filesystem {
	return &Filesystem{
		imagesDirectory:    filepath.Join(dataDirectory, "images"),
		temporaryDirectory: filepath.Join(dataDirectory, "tmp"),
	}
}

func (f *Filesystem) CreateTemporary() (*os.File, error) {
	file, err := os.CreateTemp(f.temporaryDirectory, "upload-*")
	if err != nil {
		return nil, fmt.Errorf("create upload temporary file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, fmt.Errorf("secure upload temporary file: %w", err)
	}
	return file, nil
}

func (f *Filesystem) CommitTemporary(temporaryPath, storageKey string) (string, error) {
	finalPath, err := f.resolveStorageKey(storageKey)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(finalPath); err == nil {
		return "", fmt.Errorf("storage key already exists")
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check destination: %w", err)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", fmt.Errorf("atomically commit image: %w", err)
	}
	return finalPath, nil
}

func (f *Filesystem) Open(storageKey string) (*os.File, error) {
	path, err := f.resolveStorageKey(storageKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open stored image: %w", err)
	}
	return file, nil
}

func (f *Filesystem) Exists(storageKey string) (bool, error) {
	path, err := f.resolveStorageKey(storageKey)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode().IsRegular(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat stored image: %w", err)
}

func (f *Filesystem) Remove(storageKey string) error {
	path, err := f.resolveStorageKey(storageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stored image: %w", err)
	}
	return nil
}

func (f *Filesystem) RemoveTemporary(path string) error {
	clean := filepath.Clean(path)
	relative, err := filepath.Rel(f.temporaryDirectory, clean)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return fmt.Errorf("temporary path is outside the temporary directory")
	}
	if err := os.Remove(clean); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove temporary file: %w", err)
	}
	return nil
}

func (f *Filesystem) resolveStorageKey(storageKey string) (string, error) {
	if storageKey == "" || storageKey != filepath.Base(storageKey) || strings.ContainsAny(storageKey, "/\\\x00") {
		return "", fmt.Errorf("invalid storage key")
	}
	path := filepath.Join(f.imagesDirectory, storageKey)
	if !fs.ValidPath(storageKey) {
		return "", fmt.Errorf("invalid storage key")
	}
	return path, nil
}
