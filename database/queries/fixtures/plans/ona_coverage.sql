SELECT (SELECT count(*) FROM episode_plans WHERE world_slug=$1),(SELECT count(*) FROM published_episode_scopes WHERE world_slug=$1)
