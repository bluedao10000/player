INSERT INTO affected SELECT DISTINCT work_id FROM knowledge_source_records r JOIN knowledge_sources s ON s.id=r.source_id WHERE s.name=$1
