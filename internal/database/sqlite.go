package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/leca/dt-cloudflare-images/internal/model"
	_ "modernc.org/sqlite"
)

// SQLiteDB implements Database backed by SQLite.
type SQLiteDB struct {
	db *sql.DB
}

// NewSQLiteDB opens (or creates) an SQLite database at dsn and runs migrations.
// For in-memory use pass "file::memory:?cache=shared".
func NewSQLiteDB(dsn string) (*SQLiteDB, error) {
	if !strings.Contains(dsn, "?") {
		dsn += "?_journal_mode=WAL&_busy_timeout=5000"
	} else if !strings.Contains(dsn, "_journal_mode") {
		dsn += "&_journal_mode=WAL&_busy_timeout=5000"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

// Close closes the underlying database connection.
func (s *SQLiteDB) Close() error {
	return s.db.Close()
}

// ---------------------------------------------------------------------------
// Images
// ---------------------------------------------------------------------------

func (s *SQLiteDB) CreateImage(img *model.Image) error {
	metaJSON, err := json.Marshal(img.Meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO images (account_id, id, filename, creator, meta, require_signed_urls, uploaded)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		img.AccountID, img.ID, img.Filename, img.Creator, string(metaJSON),
		boolToInt(img.RequireSignedURLs), img.Uploaded.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert image: %w", err)
	}
	return nil
}

func (s *SQLiteDB) GetImage(accountID, imageID string) (*model.Image, error) {
	row := s.db.QueryRow(`
		SELECT account_id, id, filename, creator, meta, require_signed_urls, uploaded
		FROM images WHERE account_id = ? AND id = ?`,
		accountID, imageID,
	)
	return scanImage(row)
}

func (s *SQLiteDB) ListImages(accountID string, page, perPage int) ([]*model.Image, int, error) {
	// total count
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM images WHERE account_id = ?`, accountID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count images: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := s.db.Query(`
		SELECT account_id, id, filename, creator, meta, require_signed_urls, uploaded
		FROM images WHERE account_id = ?
		ORDER BY uploaded ASC
		LIMIT ? OFFSET ?`,
		accountID, perPage, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	images, err := scanImages(rows)
	if err != nil {
		return nil, 0, err
	}
	return images, total, nil
}

func (s *SQLiteDB) UpdateImage(img *model.Image) error {
	metaJSON, err := json.Marshal(img.Meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	res, err := s.db.Exec(`
		UPDATE images SET filename = ?, meta = ?, require_signed_urls = ?
		WHERE account_id = ? AND id = ?`,
		img.Filename, string(metaJSON), boolToInt(img.RequireSignedURLs),
		img.AccountID, img.ID,
	)
	if err != nil {
		return fmt.Errorf("update image: %w", err)
	}
	return checkRowsAffected(res, "image not found")
}

func (s *SQLiteDB) DeleteImage(accountID, imageID string) error {
	res, err := s.db.Exec(`DELETE FROM images WHERE account_id = ? AND id = ?`, accountID, imageID)
	if err != nil {
		return fmt.Errorf("delete image: %w", err)
	}
	return checkRowsAffected(res, "image not found")
}

func (s *SQLiteDB) CountImages(accountID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM images WHERE account_id = ?`, accountID).Scan(&count)
	return count, err
}

// ---------------------------------------------------------------------------
// Variants
// ---------------------------------------------------------------------------

func (s *SQLiteDB) CreateVariant(v *model.Variant) error {
	_, err := s.db.Exec(`
		INSERT INTO variants (account_id, id, fit, width, height, metadata, never_require_signed_urls)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.AccountID, v.ID, v.Options.Fit, v.Options.Width, v.Options.Height,
		v.Options.Metadata, boolToInt(v.NeverRequireSignedURLs),
	)
	if err != nil {
		return fmt.Errorf("insert variant: %w", err)
	}
	return nil
}

func (s *SQLiteDB) GetVariant(accountID, variantID string) (*model.Variant, error) {
	row := s.db.QueryRow(`
		SELECT account_id, id, fit, width, height, metadata, never_require_signed_urls
		FROM variants WHERE account_id = ? AND id = ?`,
		accountID, variantID,
	)
	v := &model.Variant{}
	var neverSigned int
	err := row.Scan(&v.AccountID, &v.ID, &v.Options.Fit, &v.Options.Width,
		&v.Options.Height, &v.Options.Metadata, &neverSigned)
	if err != nil {
		return nil, fmt.Errorf("get variant: %w", err)
	}
	v.NeverRequireSignedURLs = neverSigned != 0
	return v, nil
}

func (s *SQLiteDB) ListVariants(accountID string) ([]*model.Variant, error) {
	rows, err := s.db.Query(`
		SELECT account_id, id, fit, width, height, metadata, never_require_signed_urls
		FROM variants WHERE account_id = ?
		ORDER BY id ASC`,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	defer rows.Close()

	var variants []*model.Variant
	for rows.Next() {
		v := &model.Variant{}
		var neverSigned int
		if err := rows.Scan(&v.AccountID, &v.ID, &v.Options.Fit, &v.Options.Width,
			&v.Options.Height, &v.Options.Metadata, &neverSigned); err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}
		v.NeverRequireSignedURLs = neverSigned != 0
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

func (s *SQLiteDB) UpdateVariant(v *model.Variant) error {
	res, err := s.db.Exec(`
		UPDATE variants SET fit = ?, width = ?, height = ?, metadata = ?, never_require_signed_urls = ?
		WHERE account_id = ? AND id = ?`,
		v.Options.Fit, v.Options.Width, v.Options.Height, v.Options.Metadata,
		boolToInt(v.NeverRequireSignedURLs), v.AccountID, v.ID,
	)
	if err != nil {
		return fmt.Errorf("update variant: %w", err)
	}
	return checkRowsAffected(res, "variant not found")
}

func (s *SQLiteDB) DeleteVariant(accountID, variantID string) error {
	res, err := s.db.Exec(`DELETE FROM variants WHERE account_id = ? AND id = ?`, accountID, variantID)
	if err != nil {
		return fmt.Errorf("delete variant: %w", err)
	}
	return checkRowsAffected(res, "variant not found")
}

func (s *SQLiteDB) CountVariants(accountID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM variants WHERE account_id = ?`, accountID).Scan(&count)
	return count, err
}

// ---------------------------------------------------------------------------
// Signing Keys
// ---------------------------------------------------------------------------

func (s *SQLiteDB) CreateSigningKey(key *model.SigningKey) error {
	_, err := s.db.Exec(`
		INSERT INTO signing_keys (account_id, name, value, created_at)
		VALUES (?, ?, ?, ?)`,
		key.AccountID, key.Name, key.Value, key.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert signing key: %w", err)
	}
	return nil
}

func (s *SQLiteDB) ListSigningKeys(accountID string) ([]*model.SigningKey, error) {
	rows, err := s.db.Query(`
		SELECT account_id, name, value, created_at
		FROM signing_keys WHERE account_id = ?
		ORDER BY name ASC`,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list signing keys: %w", err)
	}
	defer rows.Close()

	var keys []*model.SigningKey
	for rows.Next() {
		k := &model.SigningKey{}
		var createdStr string
		if err := rows.Scan(&k.AccountID, &k.Name, &k.Value, &createdStr); err != nil {
			return nil, fmt.Errorf("scan signing key: %w", err)
		}
		k.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *SQLiteDB) DeleteSigningKey(accountID, name string) error {
	res, err := s.db.Exec(`DELETE FROM signing_keys WHERE account_id = ? AND name = ?`, accountID, name)
	if err != nil {
		return fmt.Errorf("delete signing key: %w", err)
	}
	return checkRowsAffected(res, "signing key not found")
}

// ---------------------------------------------------------------------------
// Direct Uploads
// ---------------------------------------------------------------------------

func (s *SQLiteDB) CreateDirectUpload(du *model.DirectUpload) error {
	metaJSON, err := json.Marshal(du.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO direct_uploads (id, account_id, expiry, meta, completed)
		VALUES (?, ?, ?, ?, ?)`,
		du.ID, du.AccountID, du.Expiry.UTC().Format(time.RFC3339),
		string(metaJSON), boolToInt(du.Completed),
	)
	if err != nil {
		return fmt.Errorf("insert direct upload: %w", err)
	}
	return nil
}

func (s *SQLiteDB) GetDirectUpload(uploadID string) (*model.DirectUpload, error) {
	row := s.db.QueryRow(`
		SELECT id, account_id, expiry, meta, completed
		FROM direct_uploads WHERE id = ?`,
		uploadID,
	)

	du := &model.DirectUpload{}
	var expiryStr, metaStr string
	var completed int
	err := row.Scan(&du.ID, &du.AccountID, &expiryStr, &metaStr, &completed)
	if err != nil {
		return nil, fmt.Errorf("get direct upload: %w", err)
	}
	du.Expiry, _ = time.Parse(time.RFC3339, expiryStr)
	du.Completed = completed != 0
	if metaStr != "" {
		if err := json.Unmarshal([]byte(metaStr), &du.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal direct upload metadata: %w", err)
		}
	}
	return du, nil
}

func (s *SQLiteDB) CompleteDirectUpload(uploadID string) error {
	res, err := s.db.Exec(`UPDATE direct_uploads SET completed = 1 WHERE id = ?`, uploadID)
	if err != nil {
		return fmt.Errorf("complete direct upload: %w", err)
	}
	return checkRowsAffected(res, "direct upload not found")
}

// ---------------------------------------------------------------------------
// V2 List (cursor-based pagination)
// ---------------------------------------------------------------------------

func (s *SQLiteDB) ListImagesV2(accountID string, cursor string, perPage int, sortOrder string) ([]*model.Image, string, error) {
	order := "ASC"
	if strings.EqualFold(sortOrder, "desc") {
		order = "DESC"
	}

	var rows *sql.Rows
	var err error

	if cursor == "" {
		rows, err = s.db.Query(fmt.Sprintf(`
			SELECT account_id, id, filename, creator, meta, require_signed_urls, uploaded
			FROM images WHERE account_id = ?
			ORDER BY uploaded %s, id %s
			LIMIT ?`, order, order),
			accountID, perPage,
		)
	} else {
		// Decode cursor: it is the uploaded timestamp and id of the last item
		var cursorUploaded, cursorID string
		parts := strings.SplitN(cursor, "|", 2)
		if len(parts) == 2 {
			cursorUploaded = parts[0]
			cursorID = parts[1]
		} else {
			return nil, "", fmt.Errorf("invalid cursor")
		}

		if order == "ASC" {
			rows, err = s.db.Query(`
				SELECT account_id, id, filename, creator, meta, require_signed_urls, uploaded
				FROM images
				WHERE account_id = ? AND (uploaded > ? OR (uploaded = ? AND id > ?))
				ORDER BY uploaded ASC, id ASC
				LIMIT ?`,
				accountID, cursorUploaded, cursorUploaded, cursorID, perPage,
			)
		} else {
			rows, err = s.db.Query(`
				SELECT account_id, id, filename, creator, meta, require_signed_urls, uploaded
				FROM images
				WHERE account_id = ? AND (uploaded < ? OR (uploaded = ? AND id < ?))
				ORDER BY uploaded DESC, id DESC
				LIMIT ?`,
				accountID, cursorUploaded, cursorUploaded, cursorID, perPage,
			)
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("list images v2: %w", err)
	}
	defer rows.Close()

	images, err := scanImages(rows)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(images) == perPage {
		last := images[len(images)-1]
		nextCursor = last.Uploaded.UTC().Format(time.RFC3339Nano) + "|" + last.ID
	}

	return images, nextCursor, nil
}

// ---------------------------------------------------------------------------
// Image Metadata (for V2 filtering)
// ---------------------------------------------------------------------------

func (s *SQLiteDB) SetImageMetadata(accountID, imageID string, meta map[string]interface{}) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("SetImageMetadata: rollback failed: %v", err)
		}
	}()

	// Delete existing metadata for this image
	_, err = tx.Exec(`DELETE FROM image_metadata WHERE account_id = ? AND image_id = ?`, accountID, imageID)
	if err != nil {
		return fmt.Errorf("delete old metadata: %w", err)
	}

	// Insert new metadata
	for k, v := range meta {
		valStr := fmt.Sprintf("%v", v)
		_, err = tx.Exec(`
			INSERT INTO image_metadata (account_id, image_id, key, value)
			VALUES (?, ?, ?, ?)`,
			accountID, imageID, k, valStr,
		)
		if err != nil {
			return fmt.Errorf("insert metadata: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteDB) ListImagesWithFilter(accountID string, cursor string, perPage int, sortOrder string, key, op string, value interface{}) ([]*model.Image, string, error) {
	order := "ASC"
	if strings.EqualFold(sortOrder, "desc") {
		order = "DESC"
	}

	valStr := fmt.Sprintf("%v", value)

	var opSQL string
	switch op {
	case "eq":
		opSQL = "="
	case "ne":
		opSQL = "!="
	case "lt":
		opSQL = "<"
	case "gt":
		opSQL = ">"
	case "le", "lte":
		opSQL = "<="
	case "ge", "gte":
		opSQL = ">="
	default:
		opSQL = "="
	}

	var rows *sql.Rows
	var err error

	baseQuery := fmt.Sprintf(`
		SELECT i.account_id, i.id, i.filename, i.creator, i.meta, i.require_signed_urls, i.uploaded
		FROM images i
		INNER JOIN image_metadata m ON i.account_id = m.account_id AND i.id = m.image_id
		WHERE i.account_id = ? AND m.key = ? AND m.value %s ?`, opSQL)

	if cursor == "" {
		query := fmt.Sprintf(`%s ORDER BY i.uploaded %s, i.id %s LIMIT ?`, baseQuery, order, order)
		rows, err = s.db.Query(query, accountID, key, valStr, perPage)
	} else {
		parts := strings.SplitN(cursor, "|", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid cursor")
		}
		cursorUploaded := parts[0]
		cursorID := parts[1]

		if order == "ASC" {
			query := fmt.Sprintf(`%s AND (i.uploaded > ? OR (i.uploaded = ? AND i.id > ?))
				ORDER BY i.uploaded ASC, i.id ASC LIMIT ?`, baseQuery)
			rows, err = s.db.Query(query, accountID, key, valStr, cursorUploaded, cursorUploaded, cursorID, perPage)
		} else {
			query := fmt.Sprintf(`%s AND (i.uploaded < ? OR (i.uploaded = ? AND i.id < ?))
				ORDER BY i.uploaded DESC, i.id DESC LIMIT ?`, baseQuery)
			rows, err = s.db.Query(query, accountID, key, valStr, cursorUploaded, cursorUploaded, cursorID, perPage)
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("list images with filter: %w", err)
	}
	defer rows.Close()

	images, err := scanImages(rows)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(images) == perPage {
		last := images[len(images)-1]
		nextCursor = last.Uploaded.UTC().Format(time.RFC3339Nano) + "|" + last.ID
	}

	return images, nextCursor, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanImage(row scannable) (*model.Image, error) {
	img := &model.Image{}
	var metaStr, uploadedStr string
	var requireSigned int

	err := row.Scan(&img.AccountID, &img.ID, &img.Filename, &img.Creator, &metaStr, &requireSigned, &uploadedStr)
	if err != nil {
		return nil, fmt.Errorf("scan image: %w", err)
	}

	img.RequireSignedURLs = requireSigned != 0
	img.Uploaded, _ = time.Parse(time.RFC3339, uploadedStr)
	if metaStr != "" && metaStr != "{}" {
		if err := json.Unmarshal([]byte(metaStr), &img.Meta); err != nil {
			return nil, fmt.Errorf("unmarshal image metadata: %w", err)
		}
	}
	return img, nil
}

func scanImages(rows *sql.Rows) ([]*model.Image, error) {
	var images []*model.Image
	for rows.Next() {
		img, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func checkRowsAffected(res sql.Result, notFoundMsg string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s", notFoundMsg)
	}
	return nil
}
