INSERT INTO works(canonical_title) SELECT canonical_title FROM works WHERE id=$1 RETURNING id::text
