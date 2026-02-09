package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/leca/dt-cloudflare-images/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAccount = "test-account-001"

func newTestDB(t *testing.T) *SQLiteDB {
	t.Helper()
	db, err := NewSQLiteDB("file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateAndGetImage(t *testing.T) {
	db := newTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	img := &model.Image{
		ID:                "img-001",
		AccountID:         testAccount,
		Filename:          "photo.png",
		Meta:              map[string]interface{}{"key": "value"},
		RequireSignedURLs: true,
		Uploaded:          now,
	}

	err := db.CreateImage(img)
	require.NoError(t, err)

	got, err := db.GetImage(testAccount, "img-001")
	require.NoError(t, err)
	assert.Equal(t, img.ID, got.ID)
	assert.Equal(t, img.AccountID, got.AccountID)
	assert.Equal(t, img.Filename, got.Filename)
	assert.Equal(t, "value", got.Meta["key"])
	assert.True(t, got.RequireSignedURLs)
	assert.Equal(t, now, got.Uploaded.UTC().Truncate(time.Second))

	// not found
	_, err = db.GetImage(testAccount, "nonexistent")
	assert.Error(t, err)

	// wrong account
	_, err = db.GetImage("other-account", "img-001")
	assert.Error(t, err)
}

func TestListImagesWithPagination(t *testing.T) {
	db := newTestDB(t)

	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 25; i++ {
		img := &model.Image{
			ID:        fmt.Sprintf("img-%03d", i),
			AccountID: testAccount,
			Filename:  fmt.Sprintf("photo-%d.png", i),
			Uploaded:  base.Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, db.CreateImage(img))
	}

	// page 1
	images, total, err := db.ListImages(testAccount, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, images, 10)

	// page 2
	images, total, err = db.ListImages(testAccount, 2, 10)
	require.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, images, 10)

	// page 3 (partial)
	images, total, err = db.ListImages(testAccount, 3, 10)
	require.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, images, 5)

	// page 4 (empty)
	images, total, err = db.ListImages(testAccount, 4, 10)
	require.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, images, 0)

	// different account sees nothing
	images, total, err = db.ListImages("other-account", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Len(t, images, 0)
}

func TestUpdateImage(t *testing.T) {
	db := newTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	img := &model.Image{
		ID:                "img-upd",
		AccountID:         testAccount,
		Filename:          "old.png",
		RequireSignedURLs: false,
		Uploaded:          now,
	}
	require.NoError(t, db.CreateImage(img))

	img.Filename = "new.png"
	img.RequireSignedURLs = true
	img.Meta = map[string]interface{}{"updated": true}
	require.NoError(t, db.UpdateImage(img))

	got, err := db.GetImage(testAccount, "img-upd")
	require.NoError(t, err)
	assert.Equal(t, "new.png", got.Filename)
	assert.True(t, got.RequireSignedURLs)
	assert.Equal(t, true, got.Meta["updated"])
}

func TestDeleteImage(t *testing.T) {
	db := newTestDB(t)

	img := &model.Image{
		ID:        "img-del",
		AccountID: testAccount,
		Filename:  "delete-me.png",
		Uploaded:  time.Now().UTC(),
	}
	require.NoError(t, db.CreateImage(img))

	err := db.DeleteImage(testAccount, "img-del")
	require.NoError(t, err)

	_, err = db.GetImage(testAccount, "img-del")
	assert.Error(t, err)

	// deleting non-existent should return error
	err = db.DeleteImage(testAccount, "img-del")
	assert.Error(t, err)
}

func TestCountImages(t *testing.T) {
	db := newTestDB(t)

	count, err := db.CountImages(testAccount)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	for i := 0; i < 5; i++ {
		require.NoError(t, db.CreateImage(&model.Image{
			ID:        fmt.Sprintf("img-cnt-%d", i),
			AccountID: testAccount,
			Filename:  "f.png",
			Uploaded:  time.Now().UTC(),
		}))
	}

	count, err = db.CountImages(testAccount)
	require.NoError(t, err)
	assert.Equal(t, 5, count)

	// other account
	count, err = db.CountImages("other-account")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCreateAndGetVariant(t *testing.T) {
	db := newTestDB(t)

	v := &model.Variant{
		ID:        "hero",
		AccountID: testAccount,
		Options: model.VariantOptions{
			Fit:      "scale-down",
			Width:    1920,
			Height:   1080,
			Metadata: "none",
		},
		NeverRequireSignedURLs: true,
	}

	err := db.CreateVariant(v)
	require.NoError(t, err)

	got, err := db.GetVariant(testAccount, "hero")
	require.NoError(t, err)
	assert.Equal(t, "hero", got.ID)
	assert.Equal(t, testAccount, got.AccountID)
	assert.Equal(t, "scale-down", got.Options.Fit)
	assert.Equal(t, 1920, got.Options.Width)
	assert.Equal(t, 1080, got.Options.Height)
	assert.Equal(t, "none", got.Options.Metadata)
	assert.True(t, got.NeverRequireSignedURLs)

	// not found
	_, err = db.GetVariant(testAccount, "nonexistent")
	assert.Error(t, err)
}

func TestListVariants(t *testing.T) {
	db := newTestDB(t)

	for _, name := range []string{"thumb", "medium", "large"} {
		require.NoError(t, db.CreateVariant(&model.Variant{
			ID:        name,
			AccountID: testAccount,
			Options: model.VariantOptions{
				Fit:      "scale-down",
				Width:    100,
				Height:   100,
				Metadata: "none",
			},
		}))
	}

	variants, err := db.ListVariants(testAccount)
	require.NoError(t, err)
	assert.Len(t, variants, 3)

	// other account
	variants, err = db.ListVariants("other-account")
	require.NoError(t, err)
	assert.Len(t, variants, 0)
}

func TestDeleteVariant(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, db.CreateVariant(&model.Variant{
		ID:        "to-delete",
		AccountID: testAccount,
		Options: model.VariantOptions{
			Fit:      "contain",
			Width:    200,
			Height:   200,
			Metadata: "none",
		},
	}))

	err := db.DeleteVariant(testAccount, "to-delete")
	require.NoError(t, err)

	_, err = db.GetVariant(testAccount, "to-delete")
	assert.Error(t, err)

	// deleting non-existent should return error
	err = db.DeleteVariant(testAccount, "to-delete")
	assert.Error(t, err)
}

func TestCountVariants(t *testing.T) {
	db := newTestDB(t)

	count, err := db.CountVariants(testAccount)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	for _, name := range []string{"a", "b", "c"} {
		require.NoError(t, db.CreateVariant(&model.Variant{
			ID:        name,
			AccountID: testAccount,
			Options:   model.VariantOptions{Fit: "cover", Width: 50, Height: 50, Metadata: "none"},
		}))
	}

	count, err = db.CountVariants(testAccount)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCreateAndListSigningKeys(t *testing.T) {
	db := newTestDB(t)

	key := &model.SigningKey{
		Name:      "default",
		Value:     "secret-key-value",
		AccountID: testAccount,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, db.CreateSigningKey(key))

	keys, err := db.ListSigningKeys(testAccount)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "default", keys[0].Name)
	assert.Equal(t, "secret-key-value", keys[0].Value)

	// other account
	keys, err = db.ListSigningKeys("other-account")
	require.NoError(t, err)
	assert.Len(t, keys, 0)
}

func TestDeleteSigningKey(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, db.CreateSigningKey(&model.SigningKey{
		Name:      "temp-key",
		Value:     "some-value",
		AccountID: testAccount,
		CreatedAt: time.Now().UTC(),
	}))

	err := db.DeleteSigningKey(testAccount, "temp-key")
	require.NoError(t, err)

	keys, err := db.ListSigningKeys(testAccount)
	require.NoError(t, err)
	assert.Len(t, keys, 0)

	// deleting non-existent should return error
	err = db.DeleteSigningKey(testAccount, "temp-key")
	assert.Error(t, err)
}

func TestCreateAndGetDirectUpload(t *testing.T) {
	db := newTestDB(t)

	du := &model.DirectUpload{
		ID:        "du-001",
		AccountID: testAccount,
		Expiry:    time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second),
		Metadata:  map[string]interface{}{"source": "test"},
		Completed: false,
	}
	require.NoError(t, db.CreateDirectUpload(du))

	got, err := db.GetDirectUpload("du-001")
	require.NoError(t, err)
	assert.Equal(t, "du-001", got.ID)
	assert.Equal(t, testAccount, got.AccountID)
	assert.False(t, got.Completed)
	assert.Equal(t, "test", got.Metadata["source"])

	// not found
	_, err = db.GetDirectUpload("nonexistent")
	assert.Error(t, err)
}

func TestCompleteDirectUpload(t *testing.T) {
	db := newTestDB(t)

	du := &model.DirectUpload{
		ID:        "du-complete",
		AccountID: testAccount,
		Expiry:    time.Now().UTC().Add(30 * time.Minute),
		Completed: false,
	}
	require.NoError(t, db.CreateDirectUpload(du))

	err := db.CompleteDirectUpload("du-complete")
	require.NoError(t, err)

	got, err := db.GetDirectUpload("du-complete")
	require.NoError(t, err)
	assert.True(t, got.Completed)

	// completing non-existent should return error
	err = db.CompleteDirectUpload("nonexistent")
	assert.Error(t, err)
}

func TestListImagesV2(t *testing.T) {
	db := newTestDB(t)

	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 15; i++ {
		img := &model.Image{
			ID:        fmt.Sprintf("v2-img-%03d", i),
			AccountID: testAccount,
			Filename:  fmt.Sprintf("photo-%d.png", i),
			Uploaded:  base.Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, db.CreateImage(img))
	}

	// first page, ascending
	images, cursor, err := db.ListImagesV2(testAccount, "", 5, "asc")
	require.NoError(t, err)
	assert.Len(t, images, 5)
	assert.NotEmpty(t, cursor)
	assert.Equal(t, "v2-img-000", images[0].ID)
	assert.Equal(t, "v2-img-004", images[4].ID)

	// second page using cursor
	images2, cursor2, err := db.ListImagesV2(testAccount, cursor, 5, "asc")
	require.NoError(t, err)
	assert.Len(t, images2, 5)
	assert.NotEmpty(t, cursor2)
	assert.Equal(t, "v2-img-005", images2[0].ID)
	assert.Equal(t, "v2-img-009", images2[4].ID)

	// third page
	images3, cursor3, err := db.ListImagesV2(testAccount, cursor2, 5, "asc")
	require.NoError(t, err)
	assert.Len(t, images3, 5)
	assert.Equal(t, "v2-img-010", images3[0].ID)
	assert.Equal(t, "v2-img-014", images3[4].ID)

	// fourth page (should be empty, no cursor)
	images4, cursor4, err := db.ListImagesV2(testAccount, cursor3, 5, "asc")
	require.NoError(t, err)
	assert.Len(t, images4, 0)
	assert.Empty(t, cursor4)

	// descending order
	imagesDesc, _, err := db.ListImagesV2(testAccount, "", 5, "desc")
	require.NoError(t, err)
	assert.Len(t, imagesDesc, 5)
	assert.Equal(t, "v2-img-014", imagesDesc[0].ID)
	assert.Equal(t, "v2-img-010", imagesDesc[4].ID)
}
