SELECT version,checksum,parser_type,asset_name FROM knowledge_source_versions WHERE source_id=$1 AND version=coalesce(nullif($2,'@current'),$3) AND snapshot_key IS NOT NULL
