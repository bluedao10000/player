SELECT EXISTS(SELECT FROM knowledge_source_versions WHERE source_id=$1 AND version=$2 AND snapshot_key IS NOT NULL)
