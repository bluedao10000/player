SELECT r.id FROM reader_plan_orders o JOIN reader_identities r ON r.id=o.reader_id WHERE o.id=$1 AND o.app_id=$2 AND o.seller_id=$3 FOR UPDATE OF r
