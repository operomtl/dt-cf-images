package database

import "github.com/leca/dt-cloudflare-images/internal/model"

// Database defines the persistence interface for all domain objects.
type Database interface {
	// Images
	CreateImage(img *model.Image) error
	GetImage(accountID, imageID string) (*model.Image, error)
	ListImages(accountID string, page, perPage int) ([]*model.Image, int, error)
	UpdateImage(img *model.Image) error
	DeleteImage(accountID, imageID string) error
	CountImages(accountID string) (int, error)

	// Variants
	CreateVariant(v *model.Variant) error
	GetVariant(accountID, variantID string) (*model.Variant, error)
	ListVariants(accountID string) ([]*model.Variant, error)
	UpdateVariant(v *model.Variant) error
	DeleteVariant(accountID, variantID string) error
	CountVariants(accountID string) (int, error)

	// Signing Keys
	CreateSigningKey(key *model.SigningKey) error
	ListSigningKeys(accountID string) ([]*model.SigningKey, error)
	DeleteSigningKey(accountID, name string) error

	// Direct Uploads
	CreateDirectUpload(du *model.DirectUpload) error
	GetDirectUpload(uploadID string) (*model.DirectUpload, error)
	CompleteDirectUpload(uploadID string) error

	// V2 List
	ListImagesV2(accountID string, cursor string, perPage int, sortOrder string) ([]*model.Image, string, error)

	// Image Metadata (for V2 filtering)
	SetImageMetadata(accountID, imageID string, meta map[string]interface{}) error
	ListImagesWithFilter(accountID string, cursor string, perPage int, sortOrder string, key, op string, value interface{}) ([]*model.Image, string, error)

	Close() error
}
