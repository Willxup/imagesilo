CREATE TABLE admin (
    id TEXT PRIMARY KEY,
    singleton INTEGER NOT NULL DEFAULT 1 UNIQUE CHECK (singleton = 1),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    admin_id TEXT NOT NULL REFERENCES admin(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE api_tokens (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    scopes TEXT NOT NULL,
    expires_at INTEGER,
    status TEXT NOT NULL CHECK (status IN ('active', 'revoked')),
    created_at INTEGER NOT NULL
);

CREATE INDEX api_tokens_status_expires_at_idx ON api_tokens(status, expires_at);

CREATE TABLE images (
    id TEXT PRIMARY KEY,
    original_name TEXT NOT NULL,
    storage_key TEXT NOT NULL UNIQUE,
    extension TEXT NOT NULL,
    mime_type TEXT NOT NULL CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp', 'image/gif')),
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0),
    source_size INTEGER NOT NULL CHECK (source_size > 0),
    stored_size INTEGER NOT NULL CHECK (stored_size > 0),
    source_sha256 BLOB NOT NULL CHECK (length(source_sha256) = 32),
    stored_sha256 BLOB NOT NULL CHECK (length(stored_sha256) = 32),
    processing_summary TEXT NOT NULL,
    visibility TEXT NOT NULL CHECK (visibility IN ('public', 'private')),
    uploaded_via TEXT NOT NULL CHECK (uploaded_via IN ('admin', 'api_token', 'import')),
    uploaded_by_api_token_id TEXT REFERENCES api_tokens(id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX images_created_at_idx ON images(created_at DESC);
CREATE INDEX images_visibility_created_at_idx ON images(visibility, created_at DESC);
CREATE INDEX images_mime_type_created_at_idx ON images(mime_type, created_at DESC);

CREATE TABLE image_aliases (
    id TEXT PRIMARY KEY,
    alias_path TEXT NOT NULL UNIQUE,
    image_id TEXT NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX image_aliases_image_id_idx ON image_aliases(image_id);

CREATE TABLE app_settings (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    max_upload_bytes INTEGER NOT NULL CHECK (max_upload_bytes > 0),
    max_batch_count INTEGER NOT NULL CHECK (max_batch_count > 0),
    max_total_pixels INTEGER NOT NULL CHECK (max_total_pixels > 0),
    compression_enabled INTEGER NOT NULL CHECK (compression_enabled IN (0, 1)),
    jpeg_quality INTEGER NOT NULL CHECK (jpeg_quality BETWEEN 1 AND 100),
    webp_quality INTEGER NOT NULL CHECK (webp_quality BETWEEN 1 AND 100),
    png_compression_level INTEGER NOT NULL CHECK (png_compression_level BETWEEN 0 AND 9),
    conversion_enabled INTEGER NOT NULL CHECK (conversion_enabled IN (0, 1)),
    conversion_webp_quality INTEGER NOT NULL CHECK (conversion_webp_quality BETWEEN 1 AND 100),
    conversion_webp_lossless INTEGER NOT NULL CHECK (conversion_webp_lossless IN (0, 1)),
    default_visibility TEXT NOT NULL CHECK (default_visibility IN ('public', 'private')),
    maintenance_hour INTEGER NOT NULL CHECK (maintenance_hour BETWEEN 0 AND 23),
    updated_at INTEGER NOT NULL
);
