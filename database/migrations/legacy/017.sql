-- Reader copy for this edition. Chapter copy is used only while both episode
-- sources still match; new evidence falls back to the active research preview.
ALTER TABLE edition_reader_intros ADD COLUMN people jsonb NOT NULL DEFAULT '{}';
UPDATE edition_reader_intros SET people='{
 "spike":"斯派克在比波普号上接悬赏。他能从阿西莫夫手里顺走一瓶药，却在宠物店认错了人。等哈基姆和一只狗同时从眼前逃走，他先去接住了狗。",
 "jet":"杰特和斯派克一起靠赏金谋生。斯派克带回一只没人认领的狗，杰特先想到给它戴上追踪项圈，好顺藤摸瓜找到窃贼。到最后，也是他给狗起了名字。",
 "asimov":"阿西莫夫带着偷来的「血腥之眼」逃到提华纳。他想把药卖掉，却始终甩不掉追兵。每遇到危险，他都更依赖手里这瓶药。",
 "katerina":"卡特琳娜跟着恋人阿西莫夫逃亡。她曾拦下他，救了被袭击的斯派克。可当飞船逼近警方的封锁线，她已经看不到两个人的出路。",
 "hakim":"哈基姆偷走了研究所的实验犬，还为了躲避追捕改变了相貌。没想到装狗的手提箱转眼又被别人偷走，他只好亲自去把狗追回来。",
 "ein":"爱因是一只从研究所流落到宠物店的威尔士柯基。它看上去只是条小狗，却会自己打开车门。杰特把它带上比波普号，给它取名爱因。"
}' WHERE world_slug='cowboy-bebop-tv';
CREATE TABLE edition_reader_chapters (
 world_slug text REFERENCES edition_reader_intros, chapter text NOT NULL, sources jsonb NOT NULL,
 passages jsonb NOT NULL, PRIMARY KEY(world_slug,chapter)
);
INSERT INTO edition_reader_chapters VALUES
('cowboy-bebop-tv','asteroid-blues',
 '[{"id":"official-1","sha256":"5a7dc2beff7433515bed2dbe39cd7d020b627331c4d68041ca86bf9a20b08870"},{"id":"guide-1","sha256":"43af5a7a608adf5fbade3cd72e7cf9579746ce8b6cbad21caaf175145fc6e56a"}]',
 '[{"text":"比波普号驶向小行星殖民地提华纳。斯派克和杰特盯上了一个叫阿西莫夫的通缉犯。他带着恋人卡特琳娜四处逃亡，身边还有一批偷来的药，叫「血腥之眼」。"},
 {"block":"bar-deal","text":"两人走进酒吧，打算把药卖给酒保。生意还没做成，追兵先到了。阿西莫夫当场用了药，硬是从围攻中杀出一条路。"},
 {"text":"后来斯派克碰上卡特琳娜，直说自己是为赏金而来。阿西莫夫突然袭击他，幸亏卡特琳娜出手阻拦。两人离开时，谁也没留意到斯派克已经顺走了一瓶药。"},
 {"text":"斯派克拿这瓶药作饵，安排了另一笔交易。帮派的人再次闯进来，阿西莫夫和卡特琳娜趁乱驾船逃走。斯派克紧追在后，前面却是警方的封锁线。阿西莫夫又一次拿起了药。"},
 {"text":"卡特琳娜终于明白，他们已经逃不掉了。斯派克追近时，她朝阿西莫夫开了枪。警方随即击毁飞船，斯派克只能调转机头。"}]'),
('cowboy-bebop-tv','stray-dog-strut',
 '[{"id":"official-2","sha256":"a4b0e00370015ac3eec9ca7d93f1d321c8b518ed95a6ce04a9ea397b33b5eb0f"},{"id":"guide-2","sha256":"51ad24d91d21c2bb6243663f269b2bc0f6fc9670c542bb7a48dfdafc16c4878c"}]',
 '[{"text":"下一份悬赏把斯派克和杰特带到火星。窃贼哈基姆偷走了研究所的一只实验动物，连脸也换过了。赏金猎人要找人，研究员急着找回动物，线索都在哈基姆随身带的手提箱里。"},
 {"text":"偏偏箱子在酒吧里又被人偷了。斯派克追到宠物店，举枪对准来卖货的人，以为他就是哈基姆。箱子一开，里面竟是一只威尔士柯基。等真正的哈基姆赶来抢狗，小狗早已溜了出去。"},
 {"text":"斯派克没抓到人，倒把狗带了回来。杰特给它戴上追踪项圈，打算等哈基姆再来偷狗，就顺着信号找到他。斯派克牵着狗出门作饵，没想到研究员也有办法引它出来。"},
 {"block":"chase","text":"一声狗哨把街上的狗都引向研究员的卡车。哈基姆开车追上来，一把捞回柯基；小狗却自己打开车门，纵身跳了出去。斯派克顾不上赏金，赶紧驾驶飞船接住它。哈基姆的车被研究员的鱼叉击中，两辆车一同冲下了桥。"},
 {"text":"电视里的赏金节目后来报道哈基姆已经「自首」，还说这是一只智力经过强化的实验犬。杰特给小狗取名爱因，让它留在比波普号上。斯派克这趟出门没领到赏金，回来倒多了个船员。"}]');
