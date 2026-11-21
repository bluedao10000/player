INSERT INTO editor_tasks(request_id,world,chapter,kind,instruction,author,base,base_head,status,draft)
 SELECT gen_random_uuid(),$1,$2,'profiles','修改介绍','tester',editor_task_snapshot($1,$2,'profiles'),(SELECT task FROM editor_heads WHERE world=$1 AND chapter IS NULL),'review',$3 RETURNING id
