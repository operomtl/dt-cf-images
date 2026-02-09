package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	fs := NewFileSystem(t.TempDir())
	data := []byte("hello, image data")

	n, err := fs.Store("acct-1", "img-1", bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, int64(len(data)), n)

	// Verify the file exists on disk at the expected path.
	path := filepath.Join(fs.basePath, "acct-1", "img-1", "original")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestRetrieve(t *testing.T) {
	fs := NewFileSystem(t.TempDir())
	data := []byte("retrieve me")

	_, err := fs.Store("acct-1", "img-2", bytes.NewReader(data))
	require.NoError(t, err)

	rc, err := fs.Retrieve("acct-1", "img-2")
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestDelete(t *testing.T) {
	fs := NewFileSystem(t.TempDir())
	data := []byte("delete me")

	_, err := fs.Store("acct-1", "img-3", bytes.NewReader(data))
	require.NoError(t, err)

	err = fs.Delete("acct-1", "img-3")
	require.NoError(t, err)

	// Verify the directory is gone.
	dir := filepath.Join(fs.basePath, "acct-1", "img-3")
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "expected directory to be removed")
}

func TestExists(t *testing.T) {
	fs := NewFileSystem(t.TempDir())

	// Should not exist yet.
	exists, err := fs.Exists("acct-1", "img-4")
	require.NoError(t, err)
	assert.False(t, exists)

	// Store data.
	_, err = fs.Store("acct-1", "img-4", bytes.NewReader([]byte("exists")))
	require.NoError(t, err)

	// Should exist now.
	exists, err = fs.Exists("acct-1", "img-4")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestStoreCreatesDirectories(t *testing.T) {
	fs := NewFileSystem(t.TempDir())

	// Deeply nested account/image IDs should create all intermediate directories.
	_, err := fs.Store("deep-account", "deep-image", bytes.NewReader([]byte("nested")))
	require.NoError(t, err)

	dir := filepath.Join(fs.basePath, "deep-account", "deep-image")
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestRetrieveNotFound(t *testing.T) {
	fs := NewFileSystem(t.TempDir())

	rc, err := fs.Retrieve("no-account", "no-image")
	assert.Error(t, err)
	assert.Nil(t, rc)
	assert.Contains(t, err.Error(), "image not found")
}

func TestDeleteNotFound(t *testing.T) {
	fs := NewFileSystem(t.TempDir())

	// Deleting a non-existent image should be idempotent (no error).
	err := fs.Delete("no-account", "no-image")
	assert.NoError(t, err)
}
