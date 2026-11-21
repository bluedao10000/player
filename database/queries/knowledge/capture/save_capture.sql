INSERT INTO episode_source_captures(topic,spec,documents,captured_at)
	SELECT topic,spec,$3,$4 FROM episode_plans WHERE world_slug=$1 AND spec=$2::jsonb
	ON CONFLICT(topic) DO UPDATE SET spec=excluded.spec,documents=excluded.documents,captured_at=excluded.captured_at
	WHERE episode_source_captures.captured_at<=excluded.captured_at
