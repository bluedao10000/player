SELECT operator FROM editor_sessions WHERE hash=$1 AND expires_at>now()
