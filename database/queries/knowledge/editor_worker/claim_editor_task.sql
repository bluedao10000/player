WITH due AS (SELECT id FROM editor_tasks WHERE status IN ('evidence','drafting','checking') AND due<=now() ORDER BY due,id FOR UPDATE SKIP LOCKED LIMIT 1)
 UPDATE editor_tasks t SET lease=gen_random_uuid(),due=now()+interval '3 minutes' FROM due WHERE t.id=due.id
 RETURNING t.id,t.world,t.chapter,t.instruction,t.feedback,t.issue,t.status,t.lease,t.round,t.attempt,t.retries,t.base,t.sources,t.draft
