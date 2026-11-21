WITH added AS (INSERT INTO scope_acceptances(topic,v,rule,fingerprint,passed,checks,picks,research_run,reader_copy,edition_slug,scope_first,scope_last)
   VALUES($1,$2,$3,$4,$5,$6,$7,nullif($8,'')::uuid,$9,$10,$11,$12) ON CONFLICT DO NOTHING RETURNING id)
   SELECT id FROM added UNION ALL SELECT id FROM scope_acceptances WHERE topic=$1 AND v=$2 AND rule=$3 AND fingerprint=$4 LIMIT 1
