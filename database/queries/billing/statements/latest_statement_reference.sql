SELECT reference FROM billing_statements WHERE day=$1 AND endpoint=$2 AND provider_account=$3 AND model=$4 AND currency=$5 ORDER BY id DESC LIMIT 1
