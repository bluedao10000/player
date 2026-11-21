SELECT count(DISTINCT work_id)::text FROM knowledge_source_records WHERE active AND norm->'refs' @> '[{"provider":"mal","id":"1"}]'
