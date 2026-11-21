SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY id DESC),'[]') FROM knowledge_curation_actions a WHERE work_id=$1 OR after_value->>'work_id'=$1::uuid::text
