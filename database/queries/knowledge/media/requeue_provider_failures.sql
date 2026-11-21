UPDATE media_candidates c SET stage='generate',
	    attempt=(SELECT max(s.attempt)+1 FROM media_steps s WHERE s.slot=c.slot AND s.round=c.round AND s.step='generate'),
	    reason=coalesce((SELECT s.result->>'reason' FROM media_steps s WHERE s.slot=c.slot AND s.round=c.round-1
	        AND s.step IN ('review','recheck') AND s.result->>'decision' IN ('redraw','recrop')
	        ORDER BY s.finished_at DESC LIMIT 1),''),updated_at=now()
	  FROM episode_art_jobs a WHERE c.art_job=a.id AND a.world_slug=$1 AND c.stage='failed'
	    AND c.reason ~ '^模型服务 HTTP (401|403|429|5[0-9]{2})$'
	    AND EXISTS(SELECT FROM media_steps s WHERE s.slot=c.slot AND s.round=c.round AND s.step='generate' AND s.error=c.reason)
	    AND NOT EXISTS(SELECT FROM media_steps s WHERE s.slot=c.slot AND s.round=c.round AND s.step='generate' AND s.error IS NULL)
