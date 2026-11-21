UPDATE media_candidates SET art_job=(SELECT min(id) FROM episode_art_jobs),stage='failed',reason='模型服务 HTTP 403' WHERE slot IN (1,2);
		INSERT INTO media_steps(slot,round,step,attempt,error,finished_at) VALUES
		(1,1,'generate',1,'模型服务 HTTP 502',now()),(1,1,'generate',2,'模型服务 HTTP 403',now()),
		(2,1,'review',1,'模型服务 HTTP 403',now())
