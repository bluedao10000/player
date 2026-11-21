CREATE TABLE knowledge_sources (
 id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 name text UNIQUE NOT NULL, type text NOT NULL, repository_url text NOT NULL,
 data_url text NOT NULL, check_url text NOT NULL, asset_name text NOT NULL,
 enabled boolean NOT NULL DEFAULT true, sync_interval interval NOT NULL DEFAULT '6 hours' CHECK(sync_interval > interval '0'),
 min_retained_ratio numeric NOT NULL DEFAULT 0.8 CHECK(min_retained_ratio BETWEEN 0 AND 1),
 last_checked_at timestamptz, last_synced_at timestamptz, current_version text,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO knowledge_sources(name,type,repository_url,data_url,check_url,asset_name) VALUES
 ('anime-offline-database','github-jsonl','https://github.com/cedya77/anime-offline-database','https://github.com/cedya77/anime-offline-database/releases/download/latest/anime-offline-database.jsonl','https://api.github.com/repos/cedya77/anime-offline-database/releases/tags/latest','anime-offline-database.jsonl'),
 ('bangumi-data','npm-dist','https://github.com/bangumi-data/bangumi-data','https://registry.npmjs.org/bangumi-data','https://registry.npmjs.org/bangumi-data/latest','package/dist/data.json');

CREATE TABLE knowledge_sync_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), source_id integer NOT NULL REFERENCES knowledge_sources,
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 status text NOT NULL CHECK(status IN ('pending','running','success','failed')), skipped boolean NOT NULL DEFAULT false,
 remote_version text, local_version text, download_url text, http_status int, content_length bigint, checksum text,
 records_read integer NOT NULL DEFAULT 0, records_created integer NOT NULL DEFAULT 0, records_updated integer NOT NULL DEFAULT 0,
 records_deleted integer NOT NULL DEFAULT 0, records_unchanged integer NOT NULL DEFAULT 0, records_failed integer NOT NULL DEFAULT 0, error_message text
);
CREATE INDEX knowledge_runs_recent ON knowledge_sync_runs(source_id,started_at DESC);
CREATE TABLE knowledge_source_versions (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, source_id integer NOT NULL REFERENCES knowledge_sources,
 version text NOT NULL, checksum text NOT NULL, metadata jsonb NOT NULL, run_id uuid NOT NULL REFERENCES knowledge_sync_runs,
 UNIQUE(source_id,version)
);
CREATE TABLE staging_source_records (
 batch_id uuid NOT NULL REFERENCES knowledge_sync_runs, record_key text NOT NULL,
 raw jsonb NOT NULL, norm jsonb NOT NULL, PRIMARY KEY(batch_id,record_key)
);
CREATE TABLE works (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), canonical_title text NOT NULL, original_title text,
 type text, episodes integer, status text, start_date date,end_date date,season text,season_year int,synopsis text,
 active boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE knowledge_source_records (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,source_id integer NOT NULL REFERENCES knowledge_sources,
 record_key text NOT NULL, version_id bigint NOT NULL REFERENCES knowledge_source_versions,
 work_id uuid NOT NULL REFERENCES works,raw jsonb NOT NULL,norm jsonb NOT NULL,active boolean NOT NULL DEFAULT true,
 UNIQUE(source_id,record_key)
);
CREATE INDEX source_record_works ON knowledge_source_records(work_id) WHERE active;
CREATE INDEX source_record_refs ON knowledge_source_records USING gin((norm->'refs') jsonb_path_ops) WHERE active;
CREATE TABLE work_titles(work_id uuid NOT NULL REFERENCES works,lang text NOT NULL,title text NOT NULL,PRIMARY KEY(work_id,lang,title));
CREATE INDEX work_title_match ON work_titles(lower(title));
CREATE TABLE work_sources(work_id uuid NOT NULL REFERENCES works,provider text NOT NULL,external_id text NOT NULL,url text NOT NULL,
 PRIMARY KEY(work_id,provider,external_id,url));
CREATE INDEX work_providers ON work_sources(provider,external_id);
CREATE INDEX work_source_url ON work_sources(url);
CREATE TABLE work_tags(work_id uuid NOT NULL REFERENCES works,name text NOT NULL,PRIMARY KEY(work_id,name));
CREATE TABLE work_studios(work_id uuid NOT NULL REFERENCES works,name text NOT NULL,PRIMARY KEY(work_id,name));
CREATE TABLE work_producers(work_id uuid NOT NULL REFERENCES works,name text NOT NULL,PRIMARY KEY(work_id,name));
CREATE TABLE work_broadcasts(work_id uuid NOT NULL REFERENCES works,detail jsonb NOT NULL,PRIMARY KEY(work_id,detail));
CREATE TABLE work_related_urls(work_id uuid NOT NULL REFERENCES works,url text NOT NULL,PRIMARY KEY(work_id,url));
CREATE VIEW work_relations AS SELECT DISTINCT r.work_id,r.url,s.work_id AS related_work_id
 FROM work_related_urls r LEFT JOIN work_sources s ON s.url=r.url;
CREATE TABLE knowledge_match_candidates (
 source_record_id bigint NOT NULL REFERENCES knowledge_source_records, candidate_work_id uuid NOT NULL REFERENCES works,
 reason text NOT NULL, first_seen_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(source_record_id,candidate_work_id,reason)
);
