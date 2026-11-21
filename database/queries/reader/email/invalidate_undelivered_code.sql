UPDATE reader_email_codes SET attempts=5 WHERE email=$1 AND digest=$2
