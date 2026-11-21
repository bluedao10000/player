INSERT INTO media_steps(slot,round,step,attempt) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING
