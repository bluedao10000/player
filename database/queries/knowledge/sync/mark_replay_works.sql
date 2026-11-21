INSERT INTO affected SELECT work_id FROM knowledge_source_records WHERE source_id=(SELECT source FROM merge_context) ON CONFLICT DO NOTHING;
