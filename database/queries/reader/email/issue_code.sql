INSERT INTO reader_email_codes(email,digest) VALUES($1,$2)
 ON CONFLICT(email) DO UPDATE SET digest=excluded.digest,issued_at=now(),attempts=0
 WHERE reader_email_codes.issued_at <= now()-interval '60 seconds'
