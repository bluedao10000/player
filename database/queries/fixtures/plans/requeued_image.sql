SELECT attempt,(SELECT count(*) FROM media_steps WHERE slot=1) FROM media_candidates WHERE slot=1 AND stage='generate'
