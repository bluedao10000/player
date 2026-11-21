SELECT EXISTS(SELECT FROM visible_reader_scopes WHERE topic=$1 AND id=$2 AND reader_copy=$3::jsonb->'scope_acceptances'->0->'reader_copy' AND picks->0->>'image'=$4)
