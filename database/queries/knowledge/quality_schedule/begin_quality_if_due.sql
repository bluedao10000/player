INSERT INTO knowledge_quality_runs(status) SELECT 'running'
 WHERE EXISTS(SELECT FROM works WHERE active) AND NOT EXISTS(
 SELECT FROM (SELECT status,finished_at FROM knowledge_quality_runs ORDER BY started_at DESC LIMIT 1) last_run
 WHERE finished_at + CASE WHEN status='failed' THEN interval '15 minutes' ELSE interval '6 hours' END >now())
 RETURNING id::text
