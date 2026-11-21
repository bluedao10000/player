SELECT reader_id FROM reader_sessions WHERE digest=$1 AND digest=$2 AND expires_at>now() FOR UPDATE
