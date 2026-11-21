SELECT to_jsonb(w)->$2::text FROM works w WHERE id=$1
