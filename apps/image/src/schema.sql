CREATE TABLE IF NOT EXISTS directories (
    id TEXT PRIMARY KEY,
    parent_id TEXT REFERENCES directories(id) ON DELETE RESTRICT,
    name TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS directory_names ON directories(coalesce(parent_id, ''), name);

CREATE TABLE IF NOT EXISTS assets (
    id TEXT PRIMARY KEY,
    storage_key TEXT NOT NULL,
    name TEXT NOT NULL,
    directory_id TEXT REFERENCES directories(id) ON DELETE SET NULL,
    mime TEXT NOT NULL,
    size INTEGER NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    orientation INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    source TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}',
    visibility TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
    previous_id TEXT UNIQUE REFERENCES assets(id),
    source_asset_id TEXT REFERENCES assets(id),
    deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS asset_directory ON assets(directory_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS asset_object ON assets(storage_key);
CREATE INDEX IF NOT EXISTS asset_source ON assets(source_asset_id) WHERE deleted_at IS NULL;

CREATE VIRTUAL TABLE IF NOT EXISTS asset_search USING fts5(
    name, metadata, content='assets', content_rowid='rowid', tokenize='trigram'
);
CREATE TRIGGER IF NOT EXISTS asset_search_insert AFTER INSERT ON assets WHEN new.deleted_at IS NULL BEGIN
    INSERT INTO asset_search(rowid,name,metadata) VALUES(new.rowid,new.name,new.metadata);
END;
CREATE TRIGGER IF NOT EXISTS asset_search_update AFTER UPDATE OF name, metadata, deleted_at ON assets BEGIN
    INSERT INTO asset_search(asset_search,rowid,name,metadata)
    SELECT 'delete',old.rowid,old.name,old.metadata WHERE old.deleted_at IS NULL;
    INSERT INTO asset_search(rowid,name,metadata)
    SELECT new.rowid,new.name,new.metadata WHERE new.deleted_at IS NULL;
END;

CREATE TABLE IF NOT EXISTS published_refs (
    asset_id TEXT NOT NULL REFERENCES assets(id),
    reference TEXT NOT NULL,
    PRIMARY KEY(asset_id, reference)
);

CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    operation TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    input TEXT NOT NULL,
    asset_id TEXT REFERENCES assets(id),
    mask_id TEXT REFERENCES assets(id),
    state TEXT NOT NULL CHECK (state IN ('queued', 'running', 'succeeded', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    request_id TEXT,
    result TEXT,
    error TEXT
);
CREATE INDEX IF NOT EXISTS jobs_queue ON jobs(state, created_at, id);
CREATE INDEX IF NOT EXISTS jobs_source ON jobs(asset_id, state);
CREATE INDEX IF NOT EXISTS jobs_mask ON jobs(mask_id, state);
