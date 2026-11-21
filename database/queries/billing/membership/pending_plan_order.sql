SELECT id,plan,amount_fen,expires_at FROM reader_plan_orders WHERE reader_id=$1 AND trade_no IS NULL AND NOT closed AND expires_at>now() ORDER BY created_at DESC LIMIT 1
