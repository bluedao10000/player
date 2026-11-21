UPDATE editor_tasks SET status=$2,attempt=$3,retries=$4,sources=coalesce($5::jsonb,'[]'),draft=coalesce($6::jsonb,'[]'),issue=$7,lease=NULL,due=now() WHERE id=$1
