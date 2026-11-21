ALTER TABLE media_candidates ADD COLUMN character_slug text,
  ADD CHECK ((batch='bebop-portrait') = (character_slug IS NOT NULL));
INSERT INTO media_candidates(slot,batch,position,character_slug,scene,variant,stage)
VALUES (19,'bebop-portrait',1,'jet','杰特',1,'reference'),
       (20,'bebop-portrait',2,'asimov','阿西莫夫',1,'reference'),
       (21,'bebop-portrait',3,'katerina','卡特琳娜',1,'reference'),
       (22,'bebop-portrait',4,'hakim','阿卜杜勒·哈基姆',1,'reference');
