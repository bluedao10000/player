SELECT count(*) FILTER(WHERE meter_state='measured' AND total_tokens=0), count(*) FILTER(WHERE meter_state='partial' AND input_tokens=3 AND total_tokens IS NULL) FROM model_usage_receipts
