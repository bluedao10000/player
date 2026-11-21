INSERT INTO billing_rates(kind,endpoint,provider_account,model,currency,effective_at,unit,unit_micros,input_micros,output_micros,cached_micros,cap_micros,created_by)
  VALUES($1,$2,$3,$4,$5,$6,$7,coalesce($8,0::bigint),coalesce($9,0::bigint),coalesce($10,0::bigint),$11,$12,$13) RETURNING jsonb_build_object('id',id,'kind',kind,'effective_at',effective_at)
