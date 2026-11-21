UPDATE reader_identities SET plan=$2,plan_until=$3 WHERE id=$1 AND (plan,plan_until) IS DISTINCT FROM ($2::text,$3::timestamptz)
