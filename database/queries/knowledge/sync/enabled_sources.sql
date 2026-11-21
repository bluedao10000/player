SELECT id FROM knowledge_sources WHERE enabled AND ($1='' OR name=$1) ORDER BY id
