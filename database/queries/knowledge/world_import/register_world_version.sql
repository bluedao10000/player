INSERT INTO knowledge_source_versions(source_id,version,checksum,metadata,parser_type,asset_name,run_id)
    SELECT $1,$2,$2,jsonb_build_object('manifest',norm),$3,$4,$5 FROM staging_source_records WHERE batch_id=$5 AND record_key=$6
    ON CONFLICT(source_id,version) DO UPDATE SET checksum=excluded.checksum RETURNING id
