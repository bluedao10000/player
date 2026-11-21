SELECT count(*) FILTER(WHERE meter_state='inconsistent'), count(*) FILTER(WHERE meter_state='unreported' AND total_tokens IS NULL AND images=1) FROM model_usage_receipts
