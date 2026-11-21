WITH t AS (SELECT * FROM editor_tasks WHERE id=$1 AND status IN ('review','published','conflict') AND round::text=$2 AND attempt::text=$3 AND jsonb_array_length(draft)>0),
        edits AS (SELECT coalesce(jsonb_object_agg(d->>'key',jsonb_build_object('original',o.txt,'text',d->'text')),'{}') patch
        FROM t CROSS JOIN LATERAL jsonb_array_elements(t.draft) d JOIN LATERAL (
          SELECT b->>'text' txt FROM jsonb_array_elements(CASE WHEN jsonb_typeof(t.base#>'{chapter,reader_passages}')='array' THEN t.base#>'{chapter,reader_passages}' ELSE t.base#>'{chapter,blocks}' END)b
          WHERE 'block/'||(b->>'key')=d->>'key'
          UNION ALL SELECT s.value#>>'{scene,text}' FROM jsonb_each(t.base->'scenes') s WHERE 'scene/'||s.key=d->>'key'
        ) o ON true)
        SELECT CASE WHEN t.kind='profiles' THEN editor_profile_preview(t.base,t.draft) ELSE jsonb_build_object('chapter',editor_render(t.base->'chapter',patch),'scenes',coalesce((SELECT jsonb_object_agg(key,editor_render(value,patch)) FROM jsonb_each(t.base->'scenes')),'{}')) END FROM t,edits
