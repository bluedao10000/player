INSERT INTO editor_tasks(request_id,world,chapter,instruction,author,base,base_head,kind)
 SELECT $1,$2,$3,$4,$5,base,(SELECT task FROM editor_heads WHERE world=$2 AND chapter IS NOT DISTINCT FROM (CASE WHEN $6='chapter' THEN $3::text END)),$6 FROM editor_task_snapshot($2,$3,$6) base WHERE base IS NOT NULL
 ON CONFLICT(request_id) DO NOTHING RETURNING id
