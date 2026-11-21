-- One selected edition and episode interval per topic. Reports are immutable.
CREATE TABLE scope_acceptances (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 topic text NOT NULL REFERENCES research_topics, v bigint NOT NULL REFERENCES world_releases,
 rule integer NOT NULL, fingerprint text NOT NULL, checked_at timestamptz NOT NULL DEFAULT now(),
 passed boolean NOT NULL, checks jsonb NOT NULL, picks jsonb NOT NULL,
 UNIQUE(topic,v,rule,fingerprint), UNIQUE(topic,id)
);
CREATE TABLE scope_heads (
 topic text PRIMARY KEY REFERENCES research_topics,
 latest bigint NOT NULL, published bigint,
 FOREIGN KEY(topic,latest) REFERENCES scope_acceptances(topic,id),
 FOREIGN KEY(topic,published) REFERENCES scope_acceptances(topic,id)
);
CREATE TABLE media_withdrawals (
 slot integer REFERENCES media_candidates, round integer CHECK(round IN (1,2)),
 reason text NOT NULL, PRIMARY KEY(slot,round)
);
CREATE VIEW published_episode_scopes AS
 SELECT t.world_slug,t.first_episode,t.last_episode,a.*
 FROM scope_heads h JOIN scope_acceptances a ON a.id=h.published AND a.passed
 JOIN research_topics t ON t.id=h.topic AND t.enabled
 JOIN published_worlds w ON w.slug=t.world_slug AND w.version_id=a.v
 WHERE NOT EXISTS (SELECT FROM jsonb_array_elements(a.picks) p
   LEFT JOIN media_candidates c ON c.slot=(p->>'slot')::int AND c.round=(p->>'round')::int AND c.stage='passed'
   LEFT JOIN media_withdrawals x ON (x.slot,x.round)=(c.slot,c.round)
   WHERE c.slot IS NULL OR x.slot IS NOT NULL);
