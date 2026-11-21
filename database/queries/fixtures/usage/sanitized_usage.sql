SELECT (usage->'input_tokens_details'->>'cached_tokens')::int=3 AND NOT usage?'model_text' FROM model_calls WHERE id=$1
