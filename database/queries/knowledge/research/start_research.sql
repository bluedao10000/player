INSERT INTO research_runs(topic,v,attempt,policy,stage) VALUES($1,$2,$3,$4,'collect') RETURNING id::text
