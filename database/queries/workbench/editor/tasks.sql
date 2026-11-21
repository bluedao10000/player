SELECT jsonb_build_object('tasks',coalesce(jsonb_agg(to_jsonb(t) ORDER BY created_at DESC),'[]'),'scopes',$1::jsonb)
 FROM (SELECT id,world,chapter,kind,instruction,status,round,attempt,created_at FROM editor_tasks ORDER BY created_at DESC LIMIT 100)t
