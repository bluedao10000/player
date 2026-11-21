SELECT reader_id FROM reader_sessions WHERE digest=$1 AND expires_at>now()
