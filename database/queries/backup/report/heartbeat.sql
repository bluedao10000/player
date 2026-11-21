INSERT INTO backup_observer(observed_at) VALUES(now()) ON CONFLICT(singleton) DO UPDATE SET observed_at=excluded.observed_at
