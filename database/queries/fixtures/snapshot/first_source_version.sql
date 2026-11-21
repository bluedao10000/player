SELECT version FROM knowledge_source_versions WHERE source_id=(SELECT id FROM knowledge_sources WHERE name='anime-offline-database') ORDER BY id LIMIT 1
