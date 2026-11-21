INSERT INTO reader_email_codes(email,digest) VALUES($1,$2) ON CONFLICT(email) DO UPDATE SET attempts=0,issued_at=now(),digest=excluded.digest
