INSERT INTO reader_plan_orders(reader_id,plan,amount_fen,credits,app_id,seller_id)
 SELECT $1,id,price_fen,credits,$3,$4 FROM reader_plans WHERE id=$2 AND price_fen=$5 RETURNING id,amount_fen,expires_at
