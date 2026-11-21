SELECT metadata->'manifest' FROM knowledge_source_versions WHERE source_id=$1 AND version=coalesce(nullif($2,'@current'),$3)
