INSERT INTO knowledge_source_records(source_id,record_key,work_id,version_id,raw,norm)
 SELECT $1,$2,$3,$4,metadata->'manifest',$5 FROM knowledge_source_versions WHERE id=$4 ON CONFLICT(source_id,record_key) DO UPDATE SET version_id=excluded.version_id,raw=excluded.raw,norm=excluded.norm,active=true
