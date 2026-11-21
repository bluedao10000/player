SELECT id FROM editor_tasks WHERE id=$1 AND lease=$2 FOR UPDATE
