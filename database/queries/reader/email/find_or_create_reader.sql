INSERT INTO reader_identities(email) VALUES($1) ON CONFLICT(email) DO UPDATE SET email=excluded.email RETURNING id
