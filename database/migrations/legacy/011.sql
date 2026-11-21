CREATE TABLE research_topics (
 id text PRIMARY KEY, world_slug text NOT NULL, first_episode integer NOT NULL,
 last_episode integer NOT NULL CHECK(last_episode>=first_episode AND first_episode>0),
 enabled boolean NOT NULL DEFAULT true
);
INSERT INTO research_topics VALUES ('bebop-tv-1998-01-02','cowboy-bebop-tv',1,2,true);

CREATE TABLE research_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 topic text NOT NULL REFERENCES research_topics,
 v bigint NOT NULL REFERENCES world_releases,
 attempt integer NOT NULL CHECK(attempt BETWEEN 1 AND 3),
 stage text NOT NULL CHECK(stage IN ('collect','write','ready','failed')),
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 model text, documents jsonb, claims jsonb, error text,
 UNIQUE(topic,v,attempt),
 CHECK (stage <> 'ready' OR (documents IS NOT NULL AND claims IS NOT NULL AND finished_at IS NOT NULL))
);
CREATE UNIQUE INDEX research_one_ready ON research_runs(topic,v) WHERE stage='ready';
CREATE TABLE research_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, run_id uuid NOT NULL REFERENCES research_runs,
 at timestamptz NOT NULL DEFAULT now(), message text NOT NULL
);
CREATE INDEX research_events_run ON research_events(run_id,id);

-- Expose only validated, scoped paragraphs for the reader's current world release.
CREATE VIEW chapter_research AS
 SELECT c.v,c.slug,r.id AS run_id,r.model,
   (SELECT jsonb_agg(p.item ORDER BY p.ord)
    FROM jsonb_array_elements(r.claims) WITH ORDINALITY p(item,ord)
    WHERE (p.item->>'episode')::integer=c.episode) AS paragraphs
 FROM research_runs r JOIN research_topics t ON r.topic=t.id
 JOIN work_chapters c ON c.v=r.v AND c.episode BETWEEN t.first_episode AND t.last_episode
 WHERE r.stage='ready';
