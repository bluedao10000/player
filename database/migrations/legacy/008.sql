-- Private transport cache: conditional HTTP requests can reuse a verified document
-- even when another document in the same enrichment job changes.
CREATE TABLE world_fetch_cache (
 source_id int REFERENCES knowledge_sources, url text, etag text NOT NULL,
 modified text NOT NULL, body bytea NOT NULL, PRIMARY KEY(source_id,url)
);
