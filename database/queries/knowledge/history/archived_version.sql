SELECT snapshot_key,parser_type,v.asset_name FROM knowledge_source_versions v JOIN knowledge_sources s ON s.id=v.source_id WHERE s.name=$1 AND v.version=$2 AND snapshot_key IS NOT NULL
