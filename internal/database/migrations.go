package database

const schema = `
CREATE TABLE IF NOT EXISTS images (
    account_id TEXT NOT NULL,
    id TEXT NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    creator TEXT NOT NULL DEFAULT '',
    meta TEXT DEFAULT '{}',
    require_signed_urls INTEGER NOT NULL DEFAULT 0,
    uploaded DATETIME NOT NULL,
    file_ext TEXT NOT NULL DEFAULT '',
    file_size INTEGER NOT NULL DEFAULT 0,
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, id)
);

CREATE TABLE IF NOT EXISTS variants (
    account_id TEXT NOT NULL,
    id TEXT NOT NULL,
    fit TEXT NOT NULL DEFAULT 'scale-down',
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    metadata TEXT NOT NULL DEFAULT 'none',
    never_require_signed_urls INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, id)
);

CREATE TABLE IF NOT EXISTS signing_keys (
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    PRIMARY KEY (account_id, name)
);

CREATE TABLE IF NOT EXISTS direct_uploads (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    expiry DATETIME NOT NULL,
    meta TEXT DEFAULT '{}',
    completed INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS image_metadata (
    account_id TEXT NOT NULL,
    image_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (account_id, image_id, key)
);

CREATE INDEX IF NOT EXISTS idx_image_metadata_filter ON image_metadata (account_id, key, value);
CREATE INDEX IF NOT EXISTS idx_images_uploaded ON images (account_id, uploaded);
`
