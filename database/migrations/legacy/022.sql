-- Freeze both the scope and the reader copy. Expanding an operations topic
-- cannot promote an older acceptance to a wider episode range.
ALTER TABLE scope_acceptances ADD COLUMN edition_slug text NOT NULL DEFAULT 'cowboy-bebop-tv',
 ADD COLUMN scope_first int NOT NULL DEFAULT 1, ADD COLUMN scope_last int NOT NULL DEFAULT 2,
 ADD COLUMN research_run uuid REFERENCES research_runs,
 ADD COLUMN reader_copy jsonb NOT NULL DEFAULT '{}', ADD CHECK(scope_first>0 AND scope_last>=scope_first);
CREATE OR REPLACE VIEW published_episode_scopes AS
 SELECT a.edition_slug AS world_slug,a.scope_first AS first_episode,a.scope_last AS last_episode,a.*
 FROM scope_heads h JOIN scope_acceptances a ON a.id=h.published AND a.passed
 JOIN research_topics t ON t.id=h.topic AND t.enabled
   AND (t.world_slug,t.first_episode,t.last_episode)=(a.edition_slug,a.scope_first,a.scope_last)
 JOIN published_worlds w ON w.slug=a.edition_slug AND w.version_id=a.v
 WHERE a.research_run IS NOT NULL AND NOT EXISTS (SELECT FROM jsonb_array_elements(a.picks) p
   LEFT JOIN media_candidates c ON c.slot=(p->>'slot')::int AND c.round=(p->>'round')::int AND c.stage='passed'
   LEFT JOIN media_withdrawals x ON (x.slot,x.round)=(c.slot,c.round)
   WHERE c.slot IS NULL OR x.slot IS NOT NULL);
