UPDATE reader_identities SET import_choice=$2 WHERE id=$1 AND import_choice IS NULL
