SELECT w.revision,array_agg(a.person ORDER BY a.person) FROM published_worlds w JOIN scene_appearances a ON a.v=w.version_id WHERE a.scene='bar-visit' GROUP BY w.revision
