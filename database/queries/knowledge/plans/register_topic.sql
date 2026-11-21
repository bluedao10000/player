INSERT INTO research_topics(id,world_slug,first_episode,last_episode) VALUES($1,$2,$3,$3) ON CONFLICT DO NOTHING
