UPDATE knowledge_sources SET last_checked_at=now()-interval '7 hours' WHERE name=$1
