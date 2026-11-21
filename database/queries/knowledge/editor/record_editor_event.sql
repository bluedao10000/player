INSERT INTO editor_events(task,round,attempt,stage,note,detail) SELECT id,round,attempt,$2,$3,$4 FROM editor_tasks WHERE id=$1
