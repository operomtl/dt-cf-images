package storage

import "io"

// Storage defines the interface for image blob storage.
type Storage interface {
	// Store writes image data and returns the number of bytes written.
	Store(accountID, imageID string, data io.Reader) (int64, error)

	// Retrieve returns a ReadCloser for the stored image data.
	Retrieve(accountID, imageID string) (io.ReadCloser, error)

	// Delete removes the stored image data.
	Delete(accountID, imageID string) error

	// Exists checks whether image data exists in storage.
	Exists(accountID, imageID string) (bool, error)
}
