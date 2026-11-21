INSERT INTO editor_heads(world,chapter,task,patch) VALUES($1,$2,$3,$4) ON CONFLICT(world,chapter) DO UPDATE SET task=excluded.task,patch=excluded.patch
