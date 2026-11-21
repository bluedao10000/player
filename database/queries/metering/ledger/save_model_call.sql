INSERT INTO model_calls(id,purpose,operation,subject,endpoint,requested_model,started_at,finished_at,outcome,http_status,request_id,response_id,resolved_model,usage,input_tokens,output_tokens,total_tokens,images,provider_account,image_size,image_quality,idempotency_key)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,nullif($10,0),nullif($11,''),nullif($12,''),nullif($13,''),$14,$15,$16,$17,$18,$19,nullif($20,''),nullif($21,''),nullif($22,''))
 ON CONFLICT(id) DO UPDATE SET finished_at=excluded.finished_at,outcome=excluded.outcome,http_status=excluded.http_status,
 request_id=excluded.request_id,response_id=excluded.response_id,resolved_model=excluded.resolved_model,usage=excluded.usage,input_tokens=excluded.input_tokens,
 output_tokens=excluded.output_tokens,total_tokens=excluded.total_tokens,images=excluded.images
 WHERE model_calls.outcome='started' AND excluded.outcome<>'started'
 AND ROW(model_calls.purpose,model_calls.operation,model_calls.subject,model_calls.endpoint,model_calls.provider_account,model_calls.requested_model,model_calls.started_at)
 = ROW(excluded.purpose,excluded.operation,excluded.subject,excluded.endpoint,excluded.provider_account,excluded.requested_model,excluded.started_at)
