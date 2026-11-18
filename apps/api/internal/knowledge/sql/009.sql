-- Additional evidence provider for this work; a remote rate limit survives restarts.
CREATE TABLE world_source_policy (
 source_id int PRIMARY KEY REFERENCES knowledge_sources,
 supplement_url text NOT NULL, retry_after timestamptz
);
INSERT INTO world_source_policy(source_id,supplement_url)
 SELECT id,'https://en.wikipedia.org/wiki/List_of_Cowboy_Bebop_episodes' FROM knowledge_sources WHERE name='bebop-tv-guide';
