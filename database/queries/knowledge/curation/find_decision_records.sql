SELECT array_agg(id ORDER BY id) FROM knowledge_source_records WHERE (id=$1 OR ($2='merge' AND work_id=$3))
