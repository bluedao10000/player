SELECT pg_try_advisory_xact_lock(hashtextextended('match-ai:'||$1::text,0))
