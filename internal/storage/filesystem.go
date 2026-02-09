package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Compile-time check that FileSystem implements Storage.
var _ Storage = (*FileSystem)(nil)

// FileSystem implements Storage using the local filesystem.
// Files are stored at <basePath>/<accountID>/<imageID>/original.
type FileSystem struct {
	basePath string
}

// NewFileSystem creates a new FileSystem storage rooted at basePath.
func NewFileSystem(basePath string) *FileSystem {
	return &FileSystem{basePath: basePath}
}

// imagePath returns the directory path for a given account and image.
func (fs *FileSystem) imagePath(accountID, imageID string) string {
	return filepath.Join(fs.basePath, accountID, imageID)
}

// originalPath returns the full path to the original file for a given account and image.
func (fs *FileSystem) originalPath(accountID, imageID string) string {
	return filepath.Join(fs.imagePath(accountID, imageID), "original")
}

// Store writes data from the reader to disk using atomic write (temp file + rename).
// It returns the number of bytes written.
func (fs *FileSystem) Store(accountID, imageID string, data io.Reader) (int64, error) {
	dir := fs.imagePath(accountID, imageID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Write to a temp file in the same directory for atomic rename.
	tmp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		return 0, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up the temp file on any error path.
	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	n, err := io.Copy(tmp, data)
	if err != nil {
		tmp.Close()
		return 0, fmt.Errorf("writing data: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("closing temp file: %w", err)
	}

	dst := fs.originalPath(accountID, imageID)
	if err := os.Rename(tmpPath, dst); err != nil {
		return 0, fmt.Errorf("renaming temp file to %s: %w", dst, err)
	}

	// Rename succeeded; prevent deferred cleanup from removing the final file.
	tmpPath = ""

	return n, nil
}

// Retrieve opens the stored original file and returns an io.ReadCloser.
func (fs *FileSystem) Retrieve(accountID, imageID string) (io.ReadCloser, error) {
	path := fs.originalPath(accountID, imageID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("image not found: %s/%s", accountID, imageID)
		}
		return nil, fmt.Errorf("opening file %s: %w", path, err)
	}
	return f, nil
}

// Delete removes the entire <accountID>/<imageID>/ directory.
// It is idempotent: deleting a non-existent image returns no error.
func (fs *FileSystem) Delete(accountID, imageID string) error {
	dir := fs.imagePath(accountID, imageID)
	err := os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("removing directory %s: %w", dir, err)
	}
	return nil
}

// Exists checks whether the original file exists on disk.
func (fs *FileSystem) Exists(accountID, imageID string) (bool, error) {
	path := fs.originalPath(accountID, imageID)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("checking file %s: %w", path, err)
}
