-- Use identical schema and row counts from the dump snapshot and its restore.
SELECT format('SELECT %L || chr(9) || count(*) FROM %I.%I',
              schemaname || '.' || tablename, schemaname, tablename)
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog','information_schema') AND schemaname !~ '^pg_(temp|toast)'
ORDER BY schemaname COLLATE "C", tablename COLLATE "C" \gexec
