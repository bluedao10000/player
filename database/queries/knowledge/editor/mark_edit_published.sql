UPDATE editor_tasks SET status='published',reviewer=$2,published_at=now() WHERE id=$1
