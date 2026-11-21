SELECT count(*)::text FROM knowledge_quality_runs WHERE status='success' AND report IS NOT NULL
