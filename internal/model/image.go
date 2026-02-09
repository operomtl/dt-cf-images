package model

import "time"

// Image represents a Cloudflare Images image resource.
type Image struct {
	ID                string                 `json:"id"`
	AccountID         string                 `json:"-"`
	Filename          string                 `json:"filename"`
	Creator           string                 `json:"creator"`
	Meta              map[string]interface{} `json:"meta,omitempty"`
	RequireSignedURLs bool                   `json:"requireSignedURLs"`
	Uploaded          time.Time              `json:"uploaded"`
	Variants          []string               `json:"variants"`
	Draft             bool                   `json:"draft,omitempty"`
}

// Variant represents a named image transformation preset.
type Variant struct {
	ID                     string         `json:"id"`
	AccountID              string         `json:"-"`
	Options                VariantOptions `json:"options"`
	NeverRequireSignedURLs bool           `json:"neverRequireSignedURLs"`
}

// VariantOptions holds the transformation parameters for a variant.
type VariantOptions struct {
	Fit      string `json:"fit"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Metadata string `json:"metadata"`
}

// SigningKey represents a key used for signing image URLs.
type SigningKey struct {
	Name      string    `json:"name"`
	Value     string    `json:"value,omitempty"`
	AccountID string    `json:"-"`
	CreatedAt time.Time `json:"-"`
}

// DirectUpload represents a pending direct-upload slot.
type DirectUpload struct {
	ID        string                 `json:"id"`
	AccountID string                 `json:"-"`
	UploadURL string                 `json:"uploadURL"`
	Expiry    time.Time              `json:"-"`
	Metadata  map[string]interface{} `json:"-"`
	Completed bool                   `json:"-"`
}
