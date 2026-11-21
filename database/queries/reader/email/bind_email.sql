UPDATE reader_identities SET email=$2 WHERE id=$1 AND (email IS NULL OR email=$2)
