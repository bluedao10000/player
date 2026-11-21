INSERT INTO episode_policies VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(world_slug) DO UPDATE SET
   first_episode=excluded.first_episode,last_episode=excluded.last_episode,priority=excluded.priority,
   max_episodes=excluded.max_episodes,token_budget=excluded.token_budget,reserve_tokens=excluded.reserve_tokens
