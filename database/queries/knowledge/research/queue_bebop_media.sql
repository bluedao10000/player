INSERT INTO media_candidates(slot,batch,position,scene,variant,character_slug,stage) VALUES
			 (23,'bebop-e3',1,'赌桌上的筹码',1,NULL,'reference'),(24,'bebop-e3',2,'赌桌上的筹码',2,NULL,'reference'),(25,'bebop-e3',3,'赌桌上的筹码',3,NULL,'reference'),
			 (26,'bebop-portrait',5,'菲·瓦伦丁',1,'faye','reference'),(27,'bebop-portrait',6,'戈登',1,'gordon','reference') ON CONFLICT (batch,position) DO NOTHING
