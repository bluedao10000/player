-- Run as reader database owner. The login password is provisioned out of band.
CREATE TABLE IF NOT EXISTS reader_release_receipts (
 topic text PRIMARY KEY, origin_id bigint NOT NULL, fingerprint text NOT NULL,
 target_id bigint NOT NULL, received_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(topic,target_id) REFERENCES scope_acceptances(topic,id)
);
GRANT USAGE ON SCHEMA public TO folio_release;
GRANT SELECT ON published_worlds,edition_reader_art,visible_reader_scopes,
 published_chapters,published_scenes TO folio_release;
GRANT SELECT,INSERT ON reader_release_receipts,research_topics,research_runs,episode_extensions,
 episode_plans,episode_art_jobs,media_candidates,scope_acceptances,scope_heads TO folio_release;
GRANT USAGE ON SEQUENCE episode_plans_id_seq,episode_art_jobs_id_seq,episode_candidate_slots,
 scope_acceptances_id_seq TO folio_release;
