INSERT INTO reader_email_codes(email,digest,issued_at,attempts) VALUES('test@example.org',$1,now()-$2::interval,$3)
  ON CONFLICT(email) DO UPDATE SET digest=excluded.digest,issued_at=excluded.issued_at,attempts=excluded.attempts
