UPDATE reader_identities SET plan_until=now()-interval '1 minute' WHERE id=$1
