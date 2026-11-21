SELECT (SELECT count(*) FROM staging_source_records WHERE batch_id=$1)=$3 AND
   $3::numeric >= (SELECT count(*) FROM knowledge_source_records WHERE source_id=$2 AND active)*(SELECT min_retained_ratio FROM knowledge_sources WHERE id=$2)
