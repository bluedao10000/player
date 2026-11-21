SELECT coalesce(jsonb_agg(to_jsonb(x)),'[]') FROM
  (SELECT k.*,r.work_id,r.record_key FROM knowledge_match_candidates k JOIN knowledge_source_records r ON r.id=k.source_record_id ORDER BY k.source_record_id,k.candidate_work_id,k.reason LIMIT $1)x
