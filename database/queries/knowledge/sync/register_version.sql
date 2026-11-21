INSERT INTO knowledge_source_versions(source_id,version,checksum,metadata,run_id) VALUES($1,$2,$3,$4,$5)
  ON CONFLICT(source_id,version) DO UPDATE SET checksum=excluded.checksum WHERE knowledge_source_versions.checksum=excluded.checksum RETURNING id
