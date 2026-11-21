WITH clean AS(DELETE FROM editor_sessions WHERE expires_at<=now()) INSERT INTO editor_sessions(hash,operator) VALUES($1,$2)
