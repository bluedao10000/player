SELECT (to_jsonb(t)-ARRAY['lease','due','request_id','base'])||jsonb_build_object('base',base,'events',coalesce((SELECT jsonb_agg(to_jsonb(e)-'task' ORDER BY id) FROM editor_events e WHERE task=t.id),'[]'))
 FROM editor_tasks t WHERE id=$1
