SELECT coalesce(jsonb_agg(jsonb_build_object('checked',checked_at,'works',works)),'[]') FROM (SELECT * FROM episode_inventory_snapshots ORDER BY checked_at DESC LIMIT 1)s
