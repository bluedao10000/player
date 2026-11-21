INSERT INTO reader_sessions(digest,reader_id,expires_at) VALUES($1,$2,now()+interval '30 days')
