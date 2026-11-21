SELECT count(*),coalesce(sum(total_tokens)FILTER(WHERE receipt_rank=1 AND meter_state='measured'),0) FROM model_usage_receipts
