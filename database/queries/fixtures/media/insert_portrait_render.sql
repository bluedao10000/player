INSERT INTO media_steps(slot,round,step,finished_at,result) VALUES
		(19,1,'generate',now(),jsonb_build_object('phone','crop.jpg','reference_sha',$1::text,'public_sha',$1::text,'sha',$1::text)),
		(19,1,'review',NULL,NULL)
