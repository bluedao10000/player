DELETE FROM staging_source_records s USING knowledge_sync_runs r WHERE s.batch_id=r.id AND r.source_id=$1 AND r.status='failed'
