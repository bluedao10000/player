UPDATE work_worlds SET image=$2 WHERE slug=$1 AND image IS NULL AND EXISTS(SELECT FROM visible_reader_scopes WHERE topic=$3)
