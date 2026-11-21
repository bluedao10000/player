UPDATE reader_plan_orders SET refunded_at=coalesce(refunded_at,now()) WHERE id=$1
