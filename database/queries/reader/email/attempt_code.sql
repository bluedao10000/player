UPDATE reader_email_codes SET attempts=attempts+1 WHERE email=$1 AND attempts<5 AND issued_at>now()-interval '5 minutes' RETURNING digest
