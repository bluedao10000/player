WITH term AS (SELECT greatest($3::timestamptz,max(ends_at)) AS begins FROM reader_paid_periods WHERE reader_id=$4)
 UPDATE reader_plan_orders SET trade_no=$2,paid_at=$3,closed=false,starts_at=term.begins,ends_at=term.begins+interval '720 hours' FROM term WHERE id=$1
