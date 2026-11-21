CREATE FUNCTION match_record_evidence(r knowledge_source_records) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
 SELECT jsonb_strip_nulls(jsonb_build_object('record_id',r.id,'source_id',r.source_id,'work_id',r.work_id,
 'title',r.norm->'title','aliases',r.norm->'titles','provider_ids',r.norm->'refs','type',r.norm->'type',
 'date',r.norm->'start','year',r.norm->'year','episodes',r.norm->'episodes'))
$$;
CREATE VIEW knowledge_match_evidence AS
 SELECT k.id,k.reason,match_record_evidence(r) incoming,
 (SELECT jsonb_agg(match_record_evidence(x) ORDER BY x.id) FROM knowledge_source_records x
 WHERE x.active AND x.work_id=k.candidate_work_id) candidates FROM knowledge_match_candidates k
 JOIN knowledge_source_records r ON k.source_record_id=r.id AND r.active;
CREATE TABLE knowledge_ai_match_assessments (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,candidate_id bigint NOT NULL,
 evidence_hash text NOT NULL,provider text NOT NULL,model text NOT NULL,prompt_version text NOT NULL,
 evidence jsonb NOT NULL,recommendation jsonb NOT NULL,requested_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(candidate_id,evidence_hash,provider,model,prompt_version)
);
