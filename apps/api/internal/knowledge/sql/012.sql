-- Keep earlier previews and their audit trails when an editorial scope changes.
ALTER TABLE research_topics ADD COLUMN policy integer NOT NULL DEFAULT 1;
ALTER TABLE research_runs ADD COLUMN policy integer NOT NULL DEFAULT 1;
ALTER TABLE research_runs DROP CONSTRAINT research_runs_topic_v_attempt_key;
ALTER TABLE research_runs ADD UNIQUE(topic,v,policy,attempt);
DROP INDEX research_one_ready;
CREATE UNIQUE INDEX research_one_ready ON research_runs(topic,v,policy) WHERE stage='ready';
UPDATE research_topics SET policy=2 WHERE id='bebop-tv-1998-01-02';
CREATE OR REPLACE VIEW chapter_research AS
 SELECT c.v,c.slug,r.id AS run_id,r.model,
   (SELECT jsonb_agg(p.item ORDER BY p.ord)
    FROM jsonb_array_elements(r.claims) WITH ORDINALITY p(item,ord)
    WHERE (p.item->>'episode')::integer=c.episode) AS paragraphs
 FROM research_runs r JOIN research_topics t ON r.topic=t.id AND r.policy=t.policy
 JOIN work_chapters c ON c.v=r.v AND c.episode BETWEEN t.first_episode AND t.last_episode
 WHERE r.stage='ready';
