SELECT jsonb_build_array(
		(SELECT jsonb_agg(jsonb_build_array(attname,format_type(atttypid,atttypmod),attnotnull,atthasdef) ORDER BY attnum)
		 FROM pg_attribute WHERE attrelid='backup_failed_attempts'::regclass AND attnum>0 AND NOT attisdropped),
		(SELECT jsonb_agg(jsonb_build_array(conname,pg_get_constraintdef(oid)) ORDER BY conname)
		 FROM pg_constraint WHERE conrelid='backup_failed_attempts'::regclass),
		(SELECT jsonb_agg(pg_get_indexdef(indexrelid) ORDER BY indexrelid::regclass::text)
		 FROM pg_index WHERE indrelid='backup_failed_attempts'::regclass)
	)::text
