GRANT CONNECT ON DATABASE folio TO folio_backup_report;
GRANT USAGE ON SCHEMA public TO folio_backup_report;
GRANT SELECT, INSERT, UPDATE ON backup_runs, backup_observer TO folio_backup_report;
GRANT SELECT, INSERT ON backup_failed_attempts TO folio_backup_report;
