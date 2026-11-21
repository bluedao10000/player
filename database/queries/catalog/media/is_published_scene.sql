SELECT EXISTS(SELECT FROM visible_reader_scopes s, jsonb_array_elements(s.picks) p WHERE p->>'image'=$1)
