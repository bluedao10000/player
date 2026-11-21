SELECT world,chapter,kind,status,round,attempt,draft,base FROM editor_tasks WHERE id=$1 FOR UPDATE
