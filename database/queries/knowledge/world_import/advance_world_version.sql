UPDATE knowledge_sources SET current_version=$2,last_synced_at=now() WHERE id=$1
