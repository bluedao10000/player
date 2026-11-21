SELECT count(*) >= 100 FROM reader_email_codes WHERE issued_at > now()-interval '1 minute'
