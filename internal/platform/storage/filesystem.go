package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Filesystem struct {
	imagesDirectory     string
	migrationsDirectory string
	thumbnailsDirectory string
	temporaryDirectory  string
}

type FileEntry struct {
	Key        string
	Size       int64
	ModifiedAt time.Time
	Regular    bool
}

func NewFilesystem(dataDirectory string) *Filesystem {
	return &Filesystem{
		imagesDirectory:     filepath.Join(dataDirectory, "images"),
		migrationsDirectory: filepath.Join(dataDirectory, "migrations"),
		thumbnailsDirectory: filepath.Join(dataDirectory, "cache", "thumbnails"),
		temporaryDirectory:  filepath.Join(dataDirectory, "tmp"),
	}
}

func (f *Filesystem) OpenMigration(relativePath string) (*os.File, error) {
	if relativePath == "" || !fs.ValidPath(relativePath) || strings.ContainsAny(relativePath, "\\\x00") {
		return nil, fmt.Errorf("invalid migration path")
	}
	file, err := os.OpenInRoot(f.migrationsDirectory, relativePath)
	if err != nil {
		return nil, fmt.Errorf("open migration file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("inspect migration file: %w", err)
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("migration path is not a regular file")
	}
	return file, nil
}

func (f *Filesystem) CommitThumbnailTemporary(temporaryPath, imageID string) error {
	finalPath, err := f.resolveThumbnailKey(imageID)
	if err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return fmt.Errorf("atomically commit thumbnail: %w", err)
	}
	return nil
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
	_, err := f.resolveStorageKey(storageKey)
	if err != nil {
		return nil, err
	}
	file, err := openRegularInRoot(f.imagesDirectory, storageKey)
	if err != nil {
		return nil, fmt.Errorf("open stored image: %w", err)
	}
	return file, nil
}

func (f *Filesystem) ImagePath(storageKey string) (string, error) {
	return f.resolveStorageKey(storageKey)
}

func (f *Filesystem) OpenThumbnail(imageID string) (*os.File, error) {
	_, err := f.resolveThumbnailKey(imageID)
	if err != nil {
		return nil, err
	}
	file, err := openRegularInRoot(f.thumbnailsDirectory, imageID)
	if err != nil {
		return nil, fmt.Errorf("open thumbnail: %w", err)
	}
	return file, nil
}

func (f *Filesystem) Exists(storageKey string) (bool, error) {
	_, exists, err := f.StoredSize(storageKey)
	return exists, err
}

func (f *Filesystem) StoredSize(storageKey string) (int64, bool, error) {
	file, err := f.Open(storageKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, false, fmt.Errorf("stat stored image: %w", err)
	}
	return info.Size(), true, nil
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

func (f *Filesystem) RemoveThumbnail(imageID string) error {
	path, err := f.resolveThumbnailKey(imageID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove thumbnail: %w", err)
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

func (f *Filesystem) RemoveTemporaryKey(key string) error {
	path, err := f.resolveTemporaryKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove temporary file: %w", err)
	}
	return nil
}

func (f *Filesystem) ListImages() ([]FileEntry, error) {
	return listDirectory(f.imagesDirectory)
}

func (f *Filesystem) ListThumbnails() ([]FileEntry, error) {
	return listDirectory(f.thumbnailsDirectory)
}

func (f *Filesystem) ListTemporary() ([]FileEntry, error) {
	return listDirectory(f.temporaryDirectory)
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

func (f *Filesystem) resolveThumbnailKey(imageID string) (string, error) {
	if imageID == "" || imageID != filepath.Base(imageID) || strings.ContainsAny(imageID, "/\\\x00") || !fs.ValidPath(imageID) {
		return "", fmt.Errorf("invalid thumbnail key")
	}
	return filepath.Join(f.thumbnailsDirectory, imageID), nil
}

func (f *Filesystem) resolveTemporaryKey(key string) (string, error) {
	if key == "" || key != filepath.Base(key) || strings.ContainsAny(key, "/\\\x00") || !fs.ValidPath(key) {
		return "", fmt.Errorf("invalid temporary key")
	}
	return filepath.Join(f.temporaryDirectory, key), nil
}

func listDirectory(directory string) ([]FileEntry, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("list data directory: %w", err)
	}
	result := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect data entry: %w", err)
		}
		result = append(result, FileEntry{
			Key: entry.Name(), Size: info.Size(), ModifiedAt: info.ModTime().UTC(), Regular: info.Mode().IsRegular(),
		})
	}
	return result, nil
}

func openRegularInRoot(root, name string) (*os.File, error) {
	file, err := os.OpenInRoot(root, name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("path is not a regular file")
	}
	return file, nil
}
