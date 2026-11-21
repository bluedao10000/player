BEGIN;
-- name: lock
SELECT pg_advisory_xact_lock(82045101);
-- name: ledger-table
CREATE TABLE knowledge_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());

-- name: 001.sql
CREATE TABLE knowledge_sources (
 id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 name text UNIQUE NOT NULL, type text NOT NULL, repository_url text NOT NULL,
 data_url text NOT NULL, check_url text NOT NULL, asset_name text NOT NULL,
 enabled boolean NOT NULL DEFAULT true, sync_interval interval NOT NULL DEFAULT '6 hours' CHECK(sync_interval > interval '0'),
 min_retained_ratio numeric NOT NULL DEFAULT 0.8 CHECK(min_retained_ratio BETWEEN 0 AND 1),
 last_checked_at timestamptz, last_synced_at timestamptz, current_version text,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO knowledge_sources(name,type,repository_url,data_url,check_url,asset_name) VALUES
 ('anime-offline-database','github-jsonl','https://github.com/cedya77/anime-offline-database','https://github.com/cedya77/anime-offline-database/releases/download/latest/anime-offline-database.jsonl','https://api.github.com/repos/cedya77/anime-offline-database/releases/tags/latest','anime-offline-database.jsonl'),
 ('bangumi-data','npm-dist','https://github.com/bangumi-data/bangumi-data','https://registry.npmjs.org/bangumi-data','https://registry.npmjs.org/bangumi-data/latest','package/dist/data.json');

CREATE TABLE knowledge_sync_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), source_id integer NOT NULL REFERENCES knowledge_sources,
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 status text NOT NULL CHECK(status IN ('pending','running','success','failed')), skipped boolean NOT NULL DEFAULT false,
 remote_version text, local_version text, download_url text, http_status int, content_length bigint, checksum text,
 records_read integer NOT NULL DEFAULT 0, records_created integer NOT NULL DEFAULT 0, records_updated integer NOT NULL DEFAULT 0,
 records_deleted integer NOT NULL DEFAULT 0, records_unchanged integer NOT NULL DEFAULT 0, records_failed integer NOT NULL DEFAULT 0, error_message text
);
CREATE INDEX knowledge_runs_recent ON knowledge_sync_runs(source_id,started_at DESC);
CREATE TABLE knowledge_source_versions (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, source_id integer NOT NULL REFERENCES knowledge_sources,
 version text NOT NULL, checksum text NOT NULL, metadata jsonb NOT NULL, run_id uuid NOT NULL REFERENCES knowledge_sync_runs,
 UNIQUE(source_id,version)
);
CREATE TABLE staging_source_records (
 batch_id uuid NOT NULL REFERENCES knowledge_sync_runs, record_key text NOT NULL,
 raw jsonb NOT NULL, norm jsonb NOT NULL, PRIMARY KEY(batch_id,record_key)
);
CREATE TABLE works (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), canonical_title text NOT NULL, original_title text,
 type text, episodes integer, status text, start_date date,end_date date,season text,season_year int,synopsis text,
 active boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE knowledge_source_records (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,source_id integer NOT NULL REFERENCES knowledge_sources,
 record_key text NOT NULL, version_id bigint NOT NULL REFERENCES knowledge_source_versions,
 work_id uuid NOT NULL REFERENCES works,raw jsonb NOT NULL,norm jsonb NOT NULL,active boolean NOT NULL DEFAULT true,
 UNIQUE(source_id,record_key)
);
CREATE INDEX source_record_works ON knowledge_source_records(work_id) WHERE active;
CREATE INDEX source_record_refs ON knowledge_source_records USING gin((norm->'refs') jsonb_path_ops) WHERE active;
CREATE TABLE work_titles(work_id uuid NOT NULL REFERENCES works,lang text NOT NULL,title text NOT NULL,PRIMARY KEY(work_id,lang,title));
CREATE INDEX work_title_match ON work_titles(lower(title));
CREATE TABLE work_sources(work_id uuid NOT NULL REFERENCES works,provider text NOT NULL,external_id text NOT NULL,url text NOT NULL,
 PRIMARY KEY(work_id,provider,external_id,url));
CREATE INDEX work_providers ON work_sources(provider,external_id);
CREATE INDEX work_source_url ON work_sources(url);
CREATE TABLE work_tags(work_id uuid NOT NULL REFERENCES works,name text NOT NULL,PRIMARY KEY(work_id,name));
CREATE TABLE work_studios(work_id uuid NOT NULL REFERENCES works,name text NOT NULL,PRIMARY KEY(work_id,name));
CREATE TABLE work_producers(work_id uuid NOT NULL REFERENCES works,name text NOT NULL,PRIMARY KEY(work_id,name));
CREATE TABLE work_broadcasts(work_id uuid NOT NULL REFERENCES works,detail jsonb NOT NULL,PRIMARY KEY(work_id,detail));
CREATE TABLE work_related_urls(work_id uuid NOT NULL REFERENCES works,url text NOT NULL,PRIMARY KEY(work_id,url));
CREATE VIEW work_relations AS SELECT DISTINCT r.work_id,r.url,s.work_id AS related_work_id
 FROM work_related_urls r LEFT JOIN work_sources s ON s.url=r.url;
CREATE TABLE knowledge_match_candidates (
 source_record_id bigint NOT NULL REFERENCES knowledge_source_records, candidate_work_id uuid NOT NULL REFERENCES works,
 reason text NOT NULL, first_seen_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(source_record_id,candidate_work_id,reason)
);

-- name: 002.sql
ALTER TABLE knowledge_sync_runs ADD COLUMN skip_reason text;
UPDATE knowledge_sync_runs SET skip_reason='version_unchanged' WHERE skipped;

-- name: 003.sql
ALTER TABLE knowledge_source_versions ADD COLUMN snapshot_key text;
ALTER TABLE knowledge_source_versions ADD COLUMN parser_type text;
ALTER TABLE knowledge_source_versions ADD COLUMN asset_name text;
ALTER TABLE knowledge_sources ADD COLUMN priority int NOT NULL DEFAULT 50;
UPDATE knowledge_sources SET priority=100 WHERE type='github-jsonl';
CREATE TABLE knowledge_field_priorities (
 field text NOT NULL,source_id int REFERENCES knowledge_sources,priority int NOT NULL,PRIMARY KEY(field,source_id)
);
INSERT INTO knowledge_field_priorities SELECT f,id,200 FROM knowledge_sources CROSS JOIN unnest(ARRAY['original','start','end']) f WHERE type='npm-dist';
CREATE TABLE knowledge_curation_actions(
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,work_id uuid NOT NULL REFERENCES works,
 operator text NOT NULL,action text NOT NULL,field text NOT NULL,before_value jsonb,after_value jsonb,
 reason text NOT NULL,source text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE work_field_overrides(work_id uuid REFERENCES works,field text,value jsonb NOT NULL,PRIMARY KEY(work_id,field));
CREATE TABLE knowledge_identity_pins(record_id bigint PRIMARY KEY REFERENCES knowledge_source_records,work_id uuid NOT NULL REFERENCES works);
CREATE TABLE knowledge_match_rejections(record_id bigint REFERENCES knowledge_source_records,work_id uuid REFERENCES works,PRIMARY KEY(record_id,work_id));
ALTER TABLE knowledge_match_candidates ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY UNIQUE;
CREATE TABLE knowledge_quality_rules(rule_id text PRIMARY KEY,field text NOT NULL,severity text NOT NULL CHECK(severity IN ('error','warning','info')),reason text NOT NULL,enabled boolean NOT NULL DEFAULT true);
INSERT INTO knowledge_quality_rules VALUES
 ('required','type','error','缺少作品类型'),('empty_title','canonical_title','error','标题为空'),
 ('url','url','warning','来源链接缺少有效 HTTP(S) 地址'),('title_norm','canonical_title','info','标题空白不规范'),
 ('language','lang','info','语言标签需要规范化'),('alias','titles','warning','同一语言有重复别名'),
 ('duplicate_id','refs','warning','多个作品共用来源 ID'),('conflict','refs','error','同一作品同一 provider 有不同 ID'),
 ('date','end_date','error','结束日期早于开始日期'),('enum','type/status','warning','作品类型或状态不在支持的范围'),
 ('cover','picture','info','缺少封面来源线索'),('relation','related','warning','关联 URL 无本地作品'),
 ('suspicious','episodes','warning','集数异常，需要核对');
CREATE TABLE knowledge_quality_issues(
 entity_id uuid REFERENCES works,rule_id text REFERENCES knowledge_quality_rules,field text NOT NULL,severity text NOT NULL,
 current_value jsonb,suggested_value jsonb,source text NOT NULL,reason text NOT NULL,status text NOT NULL CHECK(status IN ('open','resolved')),
 checked_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(entity_id,rule_id)
);
CREATE TABLE knowledge_quality_scores(work_id uuid PRIMARY KEY REFERENCES works,score int NOT NULL,components jsonb NOT NULL,checked_at timestamptz NOT NULL);

-- name: 004.sql
CREATE FUNCTION match_record_evidence(r knowledge_source_records) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
 SELECT jsonb_strip_nulls(jsonb_build_object('record_id',r.id,'source_id',r.source_id,'work_id',r.work_id,
 'title',r.norm->'title','aliases',r.norm->'titles','provider_ids',r.norm->'refs','type',r.norm->'type',
 'date',r.norm->'start','year',r.norm->'year','episodes',r.norm->'episodes'))
$$;
CREATE VIEW knowledge_match_evidence AS
 SELECT k.id,k.reason,match_record_evidence(r) incoming,
 (SELECT jsonb_agg(match_record_evidence(x) ORDER BY x.id) FROM knowledge_source_records x
 WHERE x.active AND x.work_id=k.candidate_work_id) candidates FROM knowledge_match_candidates k
 JOIN knowledge_source_records r ON k.source_record_id=r.id AND r.active;
CREATE TABLE knowledge_ai_match_assessments (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,candidate_id bigint NOT NULL,
 evidence_hash text NOT NULL,provider text NOT NULL,model text NOT NULL,prompt_version text NOT NULL,
 evidence jsonb NOT NULL,recommendation jsonb NOT NULL,requested_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(candidate_id,evidence_hash,provider,model,prompt_version)
);

-- name: 005.sql
CREATE TABLE knowledge_quality_runs(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 status text NOT NULL CHECK(status IN ('running','success','failed')),
 report jsonb, error_message text
);
CREATE INDEX quality_runs_recent ON knowledge_quality_runs(started_at DESC);

-- name: 006.sql
-- Content belongs to immutable source releases. Switching a world's version publishes
-- its graph at once. A public child identity is the stable (work UUID, slug) pair.
INSERT INTO knowledge_sources(name,type,repository_url,data_url,check_url,asset_name) VALUES
 ('tidepost-original','world-original','local:worlddata','local:worlddata/tidepost.json','local:worlddata/tidepost.json','tidepost.json'),
 ('bebop-tv-guide','world-enrichment','https://www.cowboy-bebop.net/','https://api.bgm.tv/v0','https://www.cowboy-bebop.net/story/','cowboy-bebop-tv.json');
CREATE TABLE world_releases (
 version_id bigint PRIMARY KEY REFERENCES knowledge_source_versions,
 work_id uuid NOT NULL REFERENCES works, UNIQUE(work_id,version_id)
);
CREATE TABLE work_worlds (
 work_id uuid PRIMARY KEY, slug text UNIQUE NOT NULL, version_id bigint NOT NULL,
 revision bigint NOT NULL DEFAULT 1, published_at timestamptz NOT NULL DEFAULT now(),
 name text NOT NULL, edition text NOT NULL, intro text NOT NULL, cast_title text NOT NULL,
 image text, origin text NOT NULL CHECK(origin IN ('original','external')),
 FOREIGN KEY(work_id,version_id) REFERENCES world_releases(work_id,version_id)
);
CREATE TABLE home_world_slots (
 position int PRIMARY KEY CHECK(position>0), work_id uuid UNIQUE NOT NULL REFERENCES work_worlds
);
CREATE TABLE world_evidence(
 v bigint REFERENCES world_releases, key text NOT NULL, uri text NOT NULL, locator text NOT NULL, digest text NOT NULL, PRIMARY KEY(v,key)
);
CREATE TABLE work_characters(
 v bigint, slug text, name text NOT NULL, role text NOT NULL, portrait text, href text,
 position int NOT NULL, external_id text, original_name text, evidence text NOT NULL,
 PRIMARY KEY(v,slug), UNIQUE(v,external_id), FOREIGN KEY(v,evidence) REFERENCES world_evidence
);
CREATE TABLE work_chapters(
 v bigint, slug text, name text NOT NULL, label text NOT NULL, intro text NOT NULL, href text, image text,
 position int NOT NULL, episode int, external_id text, original_name text, evidence text NOT NULL,
 PRIMARY KEY(v,slug), UNIQUE(v,episode), UNIQUE(v,external_id), FOREIGN KEY(v,evidence) REFERENCES world_evidence
);
CREATE TABLE work_storylines(v bigint REFERENCES world_releases,slug text,title text NOT NULL,intro text NOT NULL, position int NOT NULL, PRIMARY KEY(v,slug));
CREATE TABLE work_storyline_chapters(v bigint,line text,chapter text,position int NOT NULL, PRIMARY KEY(v,line,chapter), UNIQUE(v,line,position),
 FOREIGN KEY(v,line) REFERENCES work_storylines, FOREIGN KEY(v,chapter) REFERENCES work_chapters);
CREATE TABLE chapter_blocks(v bigint,chapter text,key text,text text NOT NULL,position int NOT NULL, PRIMARY KEY(v,chapter,key),
 UNIQUE(v,chapter,position), FOREIGN KEY(v,chapter) REFERENCES work_chapters);
CREATE TABLE chapter_appearances(v bigint,chapter text,person text,evidence text NOT NULL, PRIMARY KEY(v,chapter,person),
 FOREIGN KEY(v,person) REFERENCES work_characters, FOREIGN KEY(v,chapter) REFERENCES work_chapters, FOREIGN KEY(v,evidence) REFERENCES world_evidence);
CREATE INDEX chapter_by_character ON chapter_appearances(v,person,chapter);
CREATE TABLE work_settings(v bigint REFERENCES world_releases,name text,PRIMARY KEY(v,name));
CREATE TABLE work_scenes(v bigint,slug text,chapter text NOT NULL,name text NOT NULL,place text NOT NULL,text text NOT NULL,
 block text NOT NULL,position int NOT NULL,evidence text NOT NULL, PRIMARY KEY(v,slug), UNIQUE(v,chapter,slug), UNIQUE(v,chapter,position),
 FOREIGN KEY(v,chapter,block) REFERENCES chapter_blocks, FOREIGN KEY(v,place) REFERENCES work_settings, FOREIGN KEY(v,evidence) REFERENCES world_evidence);
CREATE TABLE scene_appearances(v bigint,chapter text,scene text,person text,evidence text NOT NULL, PRIMARY KEY(v,scene,person),
 FOREIGN KEY(v,chapter,scene) REFERENCES work_scenes(v,chapter,slug), FOREIGN KEY(v,chapter,person) REFERENCES chapter_appearances, FOREIGN KEY(v,evidence) REFERENCES world_evidence);

-- name: 007.sql
-- All page projections join through the published pointer. Archived releases are
-- available for provenance and replay but never enter public navigation.
CREATE VIEW published_worlds AS
 SELECT w.*,jsonb_build_object('work_id',w.work_id,'slug',slug,'title',name,'edition_label',edition,'intro',intro,
 'cast_title',cast_title,'hero_asset_key',image,'href','/work/'||slug,'revision',revision,'origin',origin) card
 FROM work_worlds w JOIN works k ON k.id=w.work_id AND k.active;
CREATE VIEW published_characters AS
 SELECT c.*,jsonb_strip_nulls(jsonb_build_object('slug',c.slug,'name',c.name,'role',c.role,'portrait',portrait,
 'external_id',external_id,'original_name',original_name,'href',coalesce(c.href,'/work/'||w.slug||'/character/'||c.slug))) card
 FROM work_characters c JOIN published_worlds w ON w.version_id=c.v;
CREATE VIEW published_chapters AS
 SELECT c.*,jsonb_strip_nulls(jsonb_build_object('slug',c.slug,'title',c.name,'label',label,'intro',c.intro,'image',c.image,
 'episode',episode,'external_id',external_id,'original_name',original_name,'read_state','readable','href',coalesce(c.href,'/work/'||w.slug||'/chapter/'||c.slug))) card
 FROM work_chapters c JOIN published_worlds w ON w.version_id=c.v;
CREATE VIEW published_scenes AS
 SELECT s.*,jsonb_build_object('slug',s.slug,'title',s.name,'place',place,'chapter_slug',chapter,'href','/work/'||w.slug||'/scene/'||s.slug) card
 FROM work_scenes s JOIN published_worlds w ON w.version_id=s.v;

-- name: 008.sql
-- Private transport cache: conditional HTTP requests can reuse a verified document
-- even when another document in the same enrichment job changes.
CREATE TABLE world_fetch_cache (
 source_id int REFERENCES knowledge_sources, url text, etag text NOT NULL,
 modified text NOT NULL, body bytea NOT NULL, PRIMARY KEY(source_id,url)
);

-- name: 009.sql
-- Additional evidence provider for this work; a remote rate limit survives restarts.
CREATE TABLE world_source_policy (
 source_id int PRIMARY KEY REFERENCES knowledge_sources,
 supplement_url text NOT NULL, retry_after timestamptz
);
INSERT INTO world_source_policy(source_id,supplement_url)
 SELECT id,'https://en.wikipedia.org/wiki/List_of_Cowboy_Bebop_episodes' FROM knowledge_sources WHERE name='bebop-tv-guide';

-- name: 010.sql
-- Inventory for the current reader-visible revision. A scoped acceptance record
-- will be needed before any revision can be reported as formally accepted.
CREATE VIEW work_knowledge_status AS
SELECT k.id AS work_id, coalesce(w.name,k.canonical_title) AS title,
       k.status AS release_status, k.episodes AS total_episodes,
       w.slug, w.edition, w.revision, w.version_id, w.published_at,
       CASE WHEN EXISTS (SELECT FROM scene_appearances a WHERE a.v=w.version_id)
            THEN 'linked' ELSE 'basic' END AS maturity,
       CASE WHEN w.work_id IS NULL THEN 'catalog' ELSE 'sample' END AS visibility,
       (SELECT count(*) FROM work_characters WHERE v=w.version_id) AS characters,
       (SELECT count(DISTINCT person) FROM scene_appearances WHERE v=w.version_id) AS characters_in_scenes,
       (SELECT count(*) FROM work_chapters WHERE v=w.version_id) AS chapters,
       ARRAY(SELECT episode FROM work_chapters WHERE v=w.version_id AND episode IS NOT NULL ORDER BY episode) AS readable_episodes,
       (SELECT count(*) FROM work_storylines WHERE v=w.version_id) AS storylines,
       (SELECT count(*) FROM work_scenes WHERE v=w.version_id) AS scenes,
       (SELECT count(*) FROM work_settings WHERE v=w.version_id) AS settings,
       nullif(w.image,'') IS NOT NULL AS has_hero,
       (SELECT count(*) FROM work_characters WHERE v=w.version_id AND nullif(portrait,'') IS NOT NULL) AS portraits,
       (SELECT count(*) FROM work_chapters WHERE v=w.version_id AND nullif(image,'') IS NOT NULL) AS chapter_images
FROM works k LEFT JOIN work_worlds w ON w.work_id=k.id
WHERE k.active AND (w.version_id IS NOT NULL OR EXISTS (
  SELECT FROM knowledge_source_records r WHERE r.work_id=k.id AND r.active
));
-- scene_appearances has FKs to the evidenced chapter appearance and scene.
-- The scene in turn references an existing block in that same chapter/release.

-- name: 011.sql
CREATE TABLE research_topics (
 id text PRIMARY KEY, world_slug text NOT NULL, first_episode integer NOT NULL,
 last_episode integer NOT NULL CHECK(last_episode>=first_episode AND first_episode>0),
 enabled boolean NOT NULL DEFAULT true
);
INSERT INTO research_topics VALUES ('bebop-tv-1998-01-02','cowboy-bebop-tv',1,2,true);

CREATE TABLE research_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 topic text NOT NULL REFERENCES research_topics,
 v bigint NOT NULL REFERENCES world_releases,
 attempt integer NOT NULL CHECK(attempt BETWEEN 1 AND 3),
 stage text NOT NULL CHECK(stage IN ('collect','write','ready','failed')),
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 model text, documents jsonb, claims jsonb, error text,
 UNIQUE(topic,v,attempt),
 CHECK (stage <> 'ready' OR (documents IS NOT NULL AND claims IS NOT NULL AND finished_at IS NOT NULL))
);
CREATE UNIQUE INDEX research_one_ready ON research_runs(topic,v) WHERE stage='ready';
CREATE TABLE research_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, run_id uuid NOT NULL REFERENCES research_runs,
 at timestamptz NOT NULL DEFAULT now(), message text NOT NULL
);
CREATE INDEX research_events_run ON research_events(run_id,id);

-- Expose only validated, scoped paragraphs for the reader's current world release.
CREATE VIEW chapter_research AS
 SELECT c.v,c.slug,r.id AS run_id,r.model,
   (SELECT jsonb_agg(p.item ORDER BY p.ord)
    FROM jsonb_array_elements(r.claims) WITH ORDINALITY p(item,ord)
    WHERE (p.item->>'episode')::integer=c.episode) AS paragraphs
 FROM research_runs r JOIN research_topics t ON r.topic=t.id
 JOIN work_chapters c ON c.v=r.v AND c.episode BETWEEN t.first_episode AND t.last_episode
 WHERE r.stage='ready';

-- name: 012.sql
-- Keep earlier previews and their audit trails when an editorial scope changes.
ALTER TABLE research_topics ADD COLUMN policy integer NOT NULL DEFAULT 1;
ALTER TABLE research_runs ADD COLUMN policy integer NOT NULL DEFAULT 1;
ALTER TABLE research_runs DROP CONSTRAINT research_runs_topic_v_attempt_key;
ALTER TABLE research_runs ADD UNIQUE(topic,v,policy,attempt);
DROP INDEX research_one_ready;
CREATE UNIQUE INDEX research_one_ready ON research_runs(topic,v,policy) WHERE stage='ready';
UPDATE research_topics SET policy=2 WHERE id='bebop-tv-1998-01-02';
CREATE OR REPLACE VIEW chapter_research AS
 SELECT c.v,c.slug,r.id AS run_id,r.model,
   (SELECT jsonb_agg(p.item ORDER BY p.ord)
    FROM jsonb_array_elements(r.claims) WITH ORDINALITY p(item,ord)
    WHERE (p.item->>'episode')::integer=c.episode) AS paragraphs
 FROM research_runs r JOIN research_topics t ON r.topic=t.id AND r.policy=t.policy
 JOIN work_chapters c ON c.v=r.v AND c.episode BETWEEN t.first_episode AND t.last_episode
 WHERE r.stage='ready';

-- name: 013.sql
-- Rebuild the selected guide with explicit checks of the two climactic choices.
UPDATE research_topics SET policy=3 WHERE id='bebop-tv-1998-01-02' AND policy=2;

-- name: 014.sql
CREATE TABLE edition_reader_intros (
 world_slug text PRIMARY KEY, premise text NOT NULL, setting text NOT NULL
);
INSERT INTO edition_reader_intros VALUES ('cowboy-bebop-tv',
 '斯派克和杰特靠追捕通缉犯过日子，住处就是他们的飞船比波普号。他们追着一瓶失窃的药来到小行星上的酒吧，下一份悬赏又把两人带进火星的宠物店。',
 '人们在小行星上开酒吧，在火星上做生意。来往各地的赏金猎人跟着悬赏消息找人，抓到目标才有钱可领。对斯派克和杰特来说，比波普号既是出门做活的船，也是回去吃饭的家。');

-- name: 015.sql
CREATE TABLE media_candidates (
 slot integer PRIMARY KEY CHECK(slot BETWEEN 1 AND 9), scene text NOT NULL, variant integer NOT NULL,
 stage text NOT NULL DEFAULT 'queued' CHECK(stage IN ('queued','generate','review','recrop','recheck','passed','held','failed')),
 round integer NOT NULL DEFAULT 1 CHECK(round IN (1,2)), focus integer NOT NULL DEFAULT 40 CHECK(focus BETWEEN 0 AND 100),
 reason text NOT NULL DEFAULT '', updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(scene,variant)
);
INSERT INTO media_candidates(slot,scene,variant)
 SELECT i, (ARRAY['港口日景半身','雨夜侧脸','渡口持信'])[(i-1)/3+1],(i-1)%3+1 FROM generate_series(1,9)i;
CREATE TABLE media_steps (
 slot integer REFERENCES media_candidates, round integer NOT NULL, step text NOT NULL,
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz, result jsonb, error text,
 PRIMARY KEY(slot,round,step)
);

-- name: 016.sql
ALTER TABLE media_candidates ADD COLUMN attempt integer NOT NULL DEFAULT 1 CHECK(attempt IN (1,2));
ALTER TABLE media_steps ADD COLUMN attempt integer NOT NULL DEFAULT 1 CHECK(attempt IN (1,2));
ALTER TABLE media_steps DROP CONSTRAINT media_steps_pkey;
ALTER TABLE media_steps ADD PRIMARY KEY(slot,round,step,attempt);
-- Recover first-deployment write failures after the volume owner is initialized.
UPDATE media_candidates SET stage='generate',attempt=2,reason='',updated_at=now()
 WHERE stage='failed' AND round=1 AND reason LIKE 'EACCES:%/media/%';

-- name: 017.sql
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

-- name: 018.sql
-- slot remains the global FK for audit history. position is local to the batch.
ALTER TABLE media_candidates DROP CONSTRAINT media_candidates_slot_check;
ALTER TABLE media_candidates DROP CONSTRAINT media_candidates_stage_check;
ALTER TABLE media_candidates DROP CONSTRAINT media_candidates_scene_variant_key;
ALTER TABLE media_candidates ADD COLUMN batch text NOT NULL DEFAULT 'mio',
 ADD COLUMN position integer, ADD COLUMN reference_sha text NOT NULL DEFAULT '';
UPDATE media_candidates SET position=slot;
ALTER TABLE media_candidates ALTER COLUMN position SET NOT NULL,
 ADD UNIQUE(batch,position), ADD UNIQUE(batch,scene,variant),
 ADD CHECK(position BETWEEN 1 AND 9),
 ADD CHECK(stage IN ('reference','queued','generate','review','recrop','recheck','passed','held','failed'));
INSERT INTO media_candidates(slot,batch,position,scene,variant,stage)
 SELECT 9+i,'bebop',i,(ARRAY['药瓶赴约','宠物店开箱','街头遛狗'])[(i-1)/3+1],(i-1)%3+1,'reference'
 FROM generate_series(1,9)i;

-- name: 019.sql
-- Public reader selections survive a source reimport. Keep the digest of the
-- approved original alongside each derivative; private review assets stay private.
CREATE TABLE edition_reader_art (
    world_slug text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('work','character','chapter','scene')),
    item_slug text NOT NULL,
    image text NOT NULL CHECK (image LIKE '/art/%'),
    caption text NOT NULL,
    credit text NOT NULL,
    source_sha256 text NOT NULL CHECK (source_sha256 ~ '^[a-f0-9]{64}$'),
    PRIMARY KEY (world_slug,kind,item_slug)
);

INSERT INTO edition_reader_art (world_slug,kind,item_slug,image,caption,credit,source_sha256)
SELECT 'cowboy-bebop-tv', kind, item, '/art/bebop/' || asset || '.webp', caption, 'AI 场景插画', sha
FROM (VALUES
 ('work','cowboy-bebop-tv','tijuana','斯派克走过提华纳的旧街','b8c0bfe5395e3c12945a07d5d4bb7246da2ce55c25312e70402445846eda3868'),
 ('character','spike','spike','斯派克','f01c8cebbb354f29c3a508c9720e4a4ab9c3afad8122951a7f1a8a685c85cebb'),
 ('character','ein','ein','爱因','6641ec5466396a58800cc2a5fef568ab232fc53ab48a3db08d961603a5d314cd'),
 ('chapter','asteroid-blues','asteroid-blues','斯派克带着药瓶赴约','f01c8cebbb354f29c3a508c9720e4a4ab9c3afad8122951a7f1a8a685c85cebb'),
 ('chapter','stray-dog-strut','stray-dog-strut','宠物店里，手提箱中探出一只柯基','6641ec5466396a58800cc2a5fef568ab232fc53ab48a3db08d961603a5d314cd'),
 ('scene','bar-visit','old-town','酒吧所在的小镇，提华纳','b8c0bfe5395e3c12945a07d5d4bb7246da2ce55c25312e70402445846eda3868'),
 ('scene','chinatown-chase','chinatown','追逐开始前，斯派克牵着爱因穿过唐人街','371f71655d004373a2d5efc261c527d0bf3ba1c0bd510c71df1878b66e9d5fdb')
) AS picks(kind,item,asset,caption,sha);

CREATE OR REPLACE VIEW published_worlds AS
 SELECT w.*,jsonb_build_object('work_id',w.work_id,'slug',w.slug,'title',w.name,'edition_label',w.edition,'intro',w.intro,
 'cast_title',w.cast_title,'hero_asset_key',coalesce(a.image,w.image),'href','/work/'||w.slug,'revision',w.revision,'origin',w.origin) card
 FROM work_worlds w JOIN works k ON k.id=w.work_id AND k.active
 LEFT JOIN edition_reader_art a ON (a.world_slug,a.kind,a.item_slug)=(w.slug,'work',w.slug);

CREATE OR REPLACE VIEW published_characters AS
 SELECT c.*,jsonb_strip_nulls(jsonb_build_object('slug',c.slug,'name',c.name,'role',c.role,'portrait',coalesce(a.image,c.portrait),
 'external_id',c.external_id,'original_name',c.original_name,'href',coalesce(c.href,'/work/'||w.slug||'/character/'||c.slug))) card
 FROM work_characters c JOIN published_worlds w ON w.version_id=c.v
 LEFT JOIN edition_reader_art a ON (a.world_slug,a.kind,a.item_slug)=(w.slug,'character',c.slug);

CREATE OR REPLACE VIEW published_chapters AS
 SELECT c.*,jsonb_strip_nulls(jsonb_build_object('slug',c.slug,'title',c.name,'label',c.label,'intro',c.intro,'image',coalesce(a.image,c.image),
 'image_caption',a.caption,'image_credit',a.credit,
 'episode',c.episode,'external_id',c.external_id,'original_name',c.original_name,'read_state','readable','href',coalesce(c.href,'/work/'||w.slug||'/chapter/'||c.slug))) card
 FROM work_chapters c JOIN published_worlds w ON w.version_id=c.v
 LEFT JOIN edition_reader_art a ON (a.world_slug,a.kind,a.item_slug)=(w.slug,'chapter',c.slug);

CREATE OR REPLACE VIEW published_scenes AS
 SELECT s.*,jsonb_strip_nulls(jsonb_build_object('slug',s.slug,'title',s.name,'place',s.place,'chapter_slug',s.chapter,
 'image',a.image,'image_caption',a.caption,'image_credit',a.credit,'href','/work/'||w.slug||'/scene/'||s.slug)) card
 FROM work_scenes s JOIN published_worlds w ON w.version_id=s.v
 LEFT JOIN edition_reader_art a ON (a.world_slug,a.kind,a.item_slug)=(w.slug,'scene',s.slug);

CREATE OR REPLACE VIEW work_knowledge_status AS
SELECT k.id AS work_id, coalesce(w.name,k.canonical_title) AS title,
       k.status AS release_status, k.episodes AS total_episodes,
       w.slug, w.edition, w.revision, w.version_id, w.published_at,
       CASE WHEN EXISTS (SELECT FROM scene_appearances a WHERE a.v=w.version_id)
            THEN 'linked' ELSE 'basic' END AS maturity,
       CASE WHEN w.work_id IS NULL THEN 'catalog' ELSE 'sample' END AS visibility,
       (SELECT count(*) FROM work_characters WHERE v=w.version_id) AS characters,
       (SELECT count(DISTINCT person) FROM scene_appearances WHERE v=w.version_id) AS characters_in_scenes,
       (SELECT count(*) FROM work_chapters WHERE v=w.version_id) AS chapters,
       ARRAY(SELECT episode FROM work_chapters WHERE v=w.version_id AND episode IS NOT NULL ORDER BY episode) AS readable_episodes,
       (SELECT count(*) FROM work_storylines WHERE v=w.version_id) AS storylines,
       (SELECT count(*) FROM work_scenes WHERE v=w.version_id) AS scenes,
       (SELECT count(*) FROM work_settings WHERE v=w.version_id) AS settings,
       nullif(w.card->>'hero_asset_key','') IS NOT NULL AS has_hero,
       (SELECT count(*) FROM published_characters WHERE v=w.version_id AND nullif(card->>'portrait','') IS NOT NULL) AS portraits,
       (SELECT count(*) FROM published_chapters WHERE v=w.version_id AND nullif(card->>'image','') IS NOT NULL) AS chapter_images
FROM works k LEFT JOIN published_worlds w ON w.work_id=k.id
WHERE k.active AND (w.version_id IS NOT NULL OR EXISTS (
  SELECT FROM knowledge_source_records r WHERE r.work_id=k.id AND r.active
));

-- name: 020.sql
ALTER TABLE media_candidates ADD COLUMN character_slug text,
  ADD CHECK ((batch='bebop-portrait') = (character_slug IS NOT NULL));
INSERT INTO media_candidates(slot,batch,position,character_slug,scene,variant,stage)
VALUES (19,'bebop-portrait',1,'jet','杰特',1,'reference'),
       (20,'bebop-portrait',2,'asimov','阿西莫夫',1,'reference'),
       (21,'bebop-portrait',3,'katerina','卡特琳娜',1,'reference'),
       (22,'bebop-portrait',4,'hakim','阿卜杜勒·哈基姆',1,'reference');

-- name: 021.sql
-- One selected edition and episode interval per topic. Reports are immutable.
CREATE TABLE scope_acceptances (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 topic text NOT NULL REFERENCES research_topics, v bigint NOT NULL REFERENCES world_releases,
 rule integer NOT NULL, fingerprint text NOT NULL, checked_at timestamptz NOT NULL DEFAULT now(),
 passed boolean NOT NULL, checks jsonb NOT NULL, picks jsonb NOT NULL,
 UNIQUE(topic,v,rule,fingerprint), UNIQUE(topic,id)
);
CREATE TABLE scope_heads (
 topic text PRIMARY KEY REFERENCES research_topics,
 latest bigint NOT NULL, published bigint,
 FOREIGN KEY(topic,latest) REFERENCES scope_acceptances(topic,id),
 FOREIGN KEY(topic,published) REFERENCES scope_acceptances(topic,id)
);
CREATE TABLE media_withdrawals (
 slot integer REFERENCES media_candidates, round integer CHECK(round IN (1,2)),
 reason text NOT NULL, PRIMARY KEY(slot,round)
);
CREATE VIEW published_episode_scopes AS
 SELECT t.world_slug,t.first_episode,t.last_episode,a.*
 FROM scope_heads h JOIN scope_acceptances a ON a.id=h.published AND a.passed
 JOIN research_topics t ON t.id=h.topic AND t.enabled
 JOIN published_worlds w ON w.slug=t.world_slug AND w.version_id=a.v
 WHERE NOT EXISTS (SELECT FROM jsonb_array_elements(a.picks) p
   LEFT JOIN media_candidates c ON c.slot=(p->>'slot')::int AND c.round=(p->>'round')::int AND c.stage='passed'
   LEFT JOIN media_withdrawals x ON (x.slot,x.round)=(c.slot,c.round)
   WHERE c.slot IS NULL OR x.slot IS NOT NULL);

-- name: 022.sql
-- Freeze both the scope and the reader copy. Expanding an operations topic
-- cannot promote an older acceptance to a wider episode range.
ALTER TABLE scope_acceptances ADD COLUMN edition_slug text NOT NULL DEFAULT 'cowboy-bebop-tv',
 ADD COLUMN scope_first int NOT NULL DEFAULT 1, ADD COLUMN scope_last int NOT NULL DEFAULT 2,
 ADD COLUMN research_run uuid REFERENCES research_runs,
 ADD COLUMN reader_copy jsonb NOT NULL DEFAULT '{}', ADD CHECK(scope_first>0 AND scope_last>=scope_first);
CREATE OR REPLACE VIEW published_episode_scopes AS
 SELECT a.edition_slug AS world_slug,a.scope_first AS first_episode,a.scope_last AS last_episode,a.*
 FROM scope_heads h JOIN scope_acceptances a ON a.id=h.published AND a.passed
 JOIN research_topics t ON t.id=h.topic AND t.enabled
   AND (t.world_slug,t.first_episode,t.last_episode)=(a.edition_slug,a.scope_first,a.scope_last)
 JOIN published_worlds w ON w.slug=a.edition_slug AND w.version_id=a.v
 WHERE a.research_run IS NOT NULL AND NOT EXISTS (SELECT FROM jsonb_array_elements(a.picks) p
   LEFT JOIN media_candidates c ON c.slot=(p->>'slot')::int AND c.round=(p->>'round')::int AND c.stage='passed'
   LEFT JOIN media_withdrawals x ON (x.slot,x.round)=(c.slot,c.round)
   WHERE c.slot IS NULL OR x.slot IS NOT NULL);

-- name: 023.sql
INSERT INTO research_topics(id,world_slug,first_episode,last_episode)
VALUES ('bebop-tv-1998-03','cowboy-bebop-tv',3,3);

-- Editorial routing for the delta, separate from the immutable imported world.
CREATE TABLE episode_extensions (
 topic text PRIMARY KEY REFERENCES research_topics, slug text NOT NULL, title text NOT NULL,
 intro text NOT NULL, place text NOT NULL, scene_slug text NOT NULL, scene_name text NOT NULL,
 people jsonb NOT NULL
);
INSERT INTO episode_extensions VALUES ('bebop-tv-1998-03','honky-tonk-women','Honky Tonk Women',
 '斯派克和杰特想在太空赌场赚点钱。两枚混在一起的筹码，把他们带进了赌场老板的另一笔买卖。',
 '太空站赌场','casino-chip','赌桌上的筹码',
 '[{"slug":"spike","name":"斯派克","beat":2},{"slug":"jet","name":"杰特","beat":1},{"slug":"ein","name":"爱因","beat":3},{"slug":"faye","name":"菲·瓦伦丁","role":"身负悬赏的赌客","beat":3},{"slug":"gordon","name":"戈登","role":"太空赌场的老板","beat":4}]');

CREATE VIEW published_extensions AS
 SELECT s.*,e.slug,e.title,e.intro,e.place,e.scene_slug,e.scene_name,e.people,
 jsonb_build_object('slug',e.slug,'title',e.title,'intro',e.intro,'episode',s.first_episode,
 'label','第 '||s.first_episode||' 集','href','/work/'||s.world_slug||'/chapter/'||e.slug,'read_state','readable','image',s.picks->0->>'image') chapter,
 jsonb_build_object('slug',e.scene_slug,'title',e.scene_name,'place',e.place,'chapter_slug',e.slug,
 'href','/work/'||s.world_slug||'/scene/'||e.scene_slug,'image',s.picks->0->>'image','image_credit','AI 场景插画') scene
 FROM published_episode_scopes s JOIN episode_extensions e USING(topic)
 WHERE EXISTS(SELECT FROM published_episode_scopes prior WHERE prior.v=s.v AND prior.world_slug=s.world_slug
   AND prior.first_episode=1 AND prior.last_episode=s.first_episode-1);

CREATE VIEW visible_reader_scopes AS
 SELECT s.* FROM published_episode_scopes s WHERE s.first_episode=1 OR EXISTS(SELECT FROM published_extensions e WHERE e.id=s.id);

CREATE VIEW extension_people AS
 SELECT e.id,e.v,e.world_slug,e.slug chapter_slug,p->>'slug' slug,
 jsonb_build_object('slug',p->>'slug','name',p->>'name','role',p->>'role','portrait',a.image,
 'href','/work/'||e.world_slug||'/character/'||(p->>'slug')) card,
 e.reader_copy->e.slug->((p->>'beat')::int-1)->>'text' intro
 FROM published_extensions e CROSS JOIN jsonb_array_elements(e.people) p
 LEFT JOIN edition_reader_art a ON (a.world_slug,a.kind,a.item_slug)=(e.world_slug,'character',p->>'slug')
 WHERE p ? 'role';

-- name: 024.sql
CREATE VIEW extension_cast AS
 SELECT e.v,e.topic,e.world_slug,p->>'slug' slug,coalesce(b.card,x.card) card,ord
 FROM published_extensions e CROSS JOIN jsonb_array_elements(e.people) WITH ORDINALITY person(p,ord)
 LEFT JOIN published_characters b ON b.v=e.v AND b.slug=p->>'slug'
 LEFT JOIN extension_people x ON x.id=e.id AND x.slug=p->>'slug';
-- Increase only the new topic's writing policy. Existing paired episodes retain their evidence.
UPDATE research_topics SET policy=2 WHERE id='bebop-tv-1998-03' AND policy=1;

-- name: 025.sql
-- Keep source-world inventory intact; count only publicly readable extensions.
CREATE VIEW reader_knowledge_inventory AS
 SELECT s.work_id,s.slug,to_jsonb(s)||jsonb_build_object(
   'readable_episodes',s.readable_episodes||x.episodes,'chapters',s.chapters+x.n,
   'characters',s.characters+p.n,'portraits',s.portraits+p.portraits,'chapter_images',s.chapter_images+x.n,
   'storylines',s.storylines+x.n,'scenes',s.scenes+x.n,'settings',s.settings+x.places) AS card
 FROM work_knowledge_status s
 CROSS JOIN LATERAL (SELECT count(*) n,count(DISTINCT place) places,
  coalesce(array_agg(first_episode ORDER BY first_episode),'{}'::int[]) episodes
  FROM published_extensions WHERE v=s.version_id) x
 CROSS JOIN LATERAL (SELECT count(*) n,count(card->>'portrait') portraits FROM extension_people WHERE v=s.version_id) p;

-- name: 026.sql
CREATE TABLE episode_plans (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 topic text UNIQUE NOT NULL REFERENCES research_topics,
 world_slug text NOT NULL REFERENCES work_worlds(slug),
 number integer NOT NULL CHECK(number>0),
 spec jsonb NOT NULL,
 UNIQUE(world_slug,number), UNIQUE(world_slug,topic)
);
CREATE UNIQUE INDEX episode_plan_chapter ON episode_plans(world_slug,(spec->>'slug'));
CREATE UNIQUE INDEX episode_plan_scene ON episode_plans(world_slug,(spec->>'sceneSlug'));
CREATE TABLE episode_art_jobs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 world_slug text NOT NULL REFERENCES work_worlds(slug),
 kind text NOT NULL CHECK(kind IN ('scene','portrait')),
 slug text NOT NULL, spec jsonb NOT NULL, UNIQUE(world_slug,kind,slug)
);
CREATE SEQUENCE episode_candidate_slots START 1000;
ALTER TABLE media_candidates ADD COLUMN art_job bigint REFERENCES episode_art_jobs;
INSERT INTO knowledge_sources(name,type,repository_url,data_url,check_url,asset_name,enabled)
VALUES('episode-plans','episode-manifest','local:episode-plans','local:episode-plans','local:episode-plans','*.json',false);

CREATE OR REPLACE VIEW published_extensions AS
 SELECT s.*,e.slug,e.title,e.intro,e.place,e.scene_slug,e.scene_name,e.people,
 jsonb_build_object('slug',e.slug,'title',e.title,'intro',e.intro,'episode',s.first_episode,
 'label','第 '||s.first_episode||' 集','href','/work/'||s.world_slug||'/chapter/'||e.slug,'read_state','readable','image',s.picks->0->>'image') chapter,
 jsonb_build_object('slug',e.scene_slug,'title',e.scene_name,'place',e.place,'chapter_slug',e.slug,
 'href','/work/'||s.world_slug||'/scene/'||e.scene_slug,'image',s.picks->0->>'image','image_credit','AI 场景插画') scene
 FROM published_episode_scopes s JOIN episode_extensions e USING(topic)
 WHERE NOT EXISTS (SELECT FROM generate_series(1,s.first_episode-1) n WHERE NOT EXISTS
 (SELECT FROM published_episode_scopes prior WHERE prior.v=s.v AND prior.world_slug=s.world_slug AND n BETWEEN prior.first_episode AND prior.last_episode));

CREATE OR REPLACE VIEW extension_people AS
 SELECT DISTINCT ON (e.v,p->>'slug') e.id,e.v,e.world_slug,e.slug chapter_slug,p->>'slug' slug,
 jsonb_build_object('slug',p->>'slug','name',p->>'name','role',p->>'role','portrait',a.image,
 'href','/work/'||e.world_slug||'/character/'||(p->>'slug')) card,
 e.reader_copy->e.slug->((p->>'beat')::int-1)->>'text' intro
 FROM published_extensions e CROSS JOIN jsonb_array_elements(e.people) p
 LEFT JOIN edition_reader_art a ON (a.world_slug,a.kind,a.item_slug)=(e.world_slug,'character',p->>'slug')
 WHERE p ? 'role' ORDER BY e.v,p->>'slug',e.first_episode;
CREATE OR REPLACE VIEW extension_cast AS
 SELECT e.v,e.topic,e.world_slug,p->>'slug' slug,coalesce(b.card,x.card) card,ord
 FROM published_extensions e CROSS JOIN jsonb_array_elements(e.people) WITH ORDINALITY person(p,ord)
 LEFT JOIN published_characters b ON b.v=e.v AND b.slug=p->>'slug'
 LEFT JOIN extension_people x ON x.v=e.v AND x.slug=p->>'slug';

-- name: 027.sql
CREATE TABLE episode_plan_manifests (
 world_slug text PRIMARY KEY REFERENCES work_worlds(slug), checksum text NOT NULL,
 loaded_at timestamptz NOT NULL DEFAULT now()
);

-- name: 028.sql
CREATE OR REPLACE VIEW reader_knowledge_inventory AS
 SELECT s.work_id,s.slug,to_jsonb(s)||jsonb_build_object(
   'readable_episodes',s.readable_episodes||x.episodes,'chapters',s.chapters+x.n,
   'characters',s.characters+p.n,'portraits',s.portraits+p.portraits,'chapter_images',s.chapter_images+x.n,
   'storylines',s.storylines+x.n,'scenes',s.scenes+x.n,'settings',s.settings+x.places,
   'maturity',CASE WHEN people.n>0 THEN 'linked' ELSE s.maturity END,
   'characters_in_scenes',CASE WHEN people.n>0 THEN people.n ELSE s.characters_in_scenes END) AS card
 FROM work_knowledge_status s
 CROSS JOIN LATERAL (SELECT count(*) n,count(DISTINCT place) places,
  coalesce(array_agg(first_episode ORDER BY first_episode),'{}'::int[]) episodes
  FROM published_extensions WHERE v=s.version_id) x
 CROSS JOIN LATERAL (SELECT count(*) n,count(card->>'portrait') portraits FROM extension_people WHERE v=s.version_id) p
 CROSS JOIN LATERAL (SELECT count(DISTINCT c.slug) n FROM published_extensions e JOIN episode_plans p USING(topic)
 JOIN extension_cast c USING(topic) WHERE e.v=s.version_id AND strpos(p.spec->'art'->>'brief',c.card->>'name')>0) people;

-- name: 029.sql
-- One row per outbound HTTP attempt, independent of business transactions.
CREATE TABLE model_calls (
 id uuid PRIMARY KEY, seq bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 purpose text NOT NULL CHECK (purpose IN ('content','image','vision','match')),
 operation text NOT NULL, subject text NOT NULL, endpoint text NOT NULL,
 provider_account text NOT NULL, cost_owner text NOT NULL DEFAULT 'platform', image_size text, image_quality text,
 requested_model text NOT NULL, resolved_model text, started_at timestamptz NOT NULL,
 finished_at timestamptz, outcome text NOT NULL CHECK(outcome IN ('started','ok','provider_error','http_error','unknown')),
 http_status integer, idempotency_key text, request_id text, response_id text, usage jsonb,
 input_tokens bigint CHECK(input_tokens >= 0), output_tokens bigint CHECK(output_tokens >= 0),
 total_tokens bigint CHECK(total_tokens >= 0), images integer CHECK(images >= 0),
 CHECK((outcome='started')=(finished_at IS NULL))
);
CREATE INDEX model_calls_by_request ON model_calls(endpoint,provider_account,request_id) WHERE request_id IS NOT NULL;
CREATE INDEX model_calls_by_time ON model_calls(started_at,id);
CREATE INDEX model_calls_by_subject ON model_calls(subject,started_at);
-- A provider may replay a receipt for an idempotent retry. Retain both attempts;
-- prefer the most complete observation of the receipt when summing consumption.
CREATE VIEW model_usage_receipts AS
 WITH request_alias AS (
   SELECT endpoint,provider_account,request_id,min(response_id) AS response_alias,
          count(DISTINCT response_id)>1 AS ambiguous_request
   FROM model_calls WHERE request_id IS NOT NULL
   GROUP BY endpoint,provider_account,request_id
 ), keyed AS (
   SELECT c.*,coalesce('response:'||c.response_id,'response:'||a.response_alias,
         'request:'||c.request_id,'attempt:'||c.id::text) receipt_key,
         coalesce(a.ambiguous_request,false) ambiguous_request
   FROM model_calls c LEFT JOIN request_alias a USING(endpoint,provider_account,request_id)
 ), ranked AS (
   SELECT k.*,min(started_at) OVER receipt AS receipt_started_at,
     bool_or(ambiguous_request OR (total_tokens IS NOT NULL AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL
       AND total_tokens::numeric<>input_tokens::numeric+output_tokens)) OVER receipt
      OR min(total_tokens) OVER receipt IS DISTINCT FROM max(total_tokens) OVER receipt
      OR min(input_tokens) OVER receipt IS DISTINCT FROM max(input_tokens) OVER receipt
      OR min(output_tokens) OVER receipt IS DISTINCT FROM max(output_tokens) OVER receipt
      OR min(images) OVER receipt IS DISTINCT FROM max(images) OVER receipt
      OR min(resolved_model) OVER receipt IS DISTINCT FROM max(resolved_model) OVER receipt AS conflict,
     row_number() OVER (receipt ORDER BY (total_tokens IS NOT NULL) DESC,
       (input_tokens IS NOT NULL AND output_tokens IS NOT NULL) DESC,(outcome='ok') DESC,started_at,id) receipt_rank
   FROM keyed k WINDOW receipt AS (PARTITION BY endpoint,provider_account,receipt_key)
 )
 SELECT r.*, CASE
   WHEN conflict THEN 'inconsistent'
   WHEN total_tokens IS NOT NULL THEN 'measured'
   WHEN input_tokens IS NOT NULL OR output_tokens IS NOT NULL THEN 'partial'
   WHEN outcome IN ('unknown','started') THEN 'unresolved'
   ELSE 'unreported' END AS meter_state
 FROM ranked r;

-- name: 030.sql
-- Monetary amounts are integer millionths of one currency unit. No live prices are seeded.
CREATE TABLE billing_accounts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 customer_ref text NOT NULL UNIQUE CHECK (length(customer_ref) BETWEEN 1 AND 150),
 token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
 currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
 created_by text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE billing_rates (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 kind text NOT NULL CHECK (kind IN ('retail','cost')),
 endpoint text NOT NULL CHECK (length(endpoint) BETWEEN 1 AND 512),
 provider_account text NOT NULL CHECK (length(provider_account) BETWEEN 1 AND 128),
 model text NOT NULL CHECK (length(model) BETWEEN 1 AND 200),
 currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
 effective_at timestamptz NOT NULL,
 unit text NOT NULL CHECK (unit IN ('total_tokens','split_tokens','image')),
 unit_micros bigint NOT NULL DEFAULT 0 CHECK (unit_micros>=0),
 input_micros bigint NOT NULL DEFAULT 0 CHECK (input_micros>=0),
 output_micros bigint NOT NULL DEFAULT 0 CHECK (output_micros>=0),
 cached_micros bigint CHECK (cached_micros>=0),
 cap_micros bigint NOT NULL DEFAULT 0,
 created_by text NOT NULL,
 CHECK ((kind='cost' AND cap_micros=0) OR (kind='retail' AND cap_micros>0)),
 CHECK ((unit='split_tokens' AND unit_micros=0) OR (unit<>'split_tokens' AND input_micros=0 AND output_micros=0 AND cached_micros IS NULL)),
 UNIQUE(kind,currency,endpoint,provider_account,model,effective_at)
);
-- The first retail product is a single private art preview per order.
CREATE UNIQUE INDEX billing_retail_version ON billing_rates(currency,effective_at) WHERE kind='retail';

CREATE TABLE billing_orders (
 seq bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), account_id uuid NOT NULL REFERENCES billing_accounts,
 client_key text NOT NULL CHECK(length(client_key) BETWEEN 1 AND 128),
 rate_id uuid NOT NULL REFERENCES billing_rates, request jsonb NOT NULL,
 status text NOT NULL DEFAULT 'reserved' CHECK(status IN ('reserved','running','delivered','settled','void','refunded')),
 receipt_id uuid UNIQUE REFERENCES model_calls, charge_micros bigint CHECK(charge_micros>=0),
 units jsonb, delivery_sha256 text CHECK(delivery_sha256 ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz, reason text, next_check_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(account_id,client_key)
);
CREATE INDEX billing_open_orders ON billing_orders(account_id) WHERE status IN ('reserved','running','delivered');
CREATE INDEX billing_recovery_due ON billing_orders(next_check_at) WHERE status IN ('running','delivered');
CREATE TABLE billing_entries (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, account_id uuid NOT NULL REFERENCES billing_accounts,
 kind text NOT NULL CHECK(kind IN ('credit','charge','refund')), amount_micros bigint NOT NULL,
 reference text NOT NULL CHECK(length(reference) BETWEEN 1 AND 128),
 order_id uuid REFERENCES billing_orders, actor text NOT NULL, at timestamptz NOT NULL DEFAULT now(),
 CHECK ((kind='credit' AND order_id IS NULL AND amount_micros>0) OR
        (kind='charge' AND order_id IS NOT NULL AND amount_micros<=0) OR
        (kind='refund' AND order_id IS NOT NULL AND amount_micros>=0)),
 UNIQUE(account_id,kind,reference), UNIQUE(order_id,kind)
);
CREATE UNIQUE INDEX billing_credit_source ON billing_entries(reference) WHERE kind='credit';
CREATE TABLE billing_statements (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, recorded_at timestamptz NOT NULL DEFAULT now(), replaces text NOT NULL DEFAULT '',
 day date NOT NULL, endpoint text NOT NULL, provider_account text NOT NULL, model text NOT NULL,
 currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'), amount_micros bigint NOT NULL CHECK(amount_micros>=0),
 reference text NOT NULL CHECK(length(reference) BETWEEN 1 AND 150), created_by text NOT NULL,
 UNIQUE(day,endpoint,provider_account,model,currency,reference)
);
CREATE FUNCTION billing_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'financial history is append-only';
END $$;
CREATE TRIGGER billing_prices_immutable BEFORE UPDATE OR DELETE ON billing_rates FOR EACH ROW EXECUTE FUNCTION billing_append_only();
CREATE TRIGGER billing_entries_immutable BEFORE UPDATE OR DELETE ON billing_entries FOR EACH ROW EXECUTE FUNCTION billing_append_only();
CREATE TRIGGER billing_statements_immutable BEFORE UPDATE OR DELETE ON billing_statements FOR EACH ROW EXECUTE FUNCTION billing_append_only();

CREATE VIEW billing_balances AS
SELECT a.id,a.customer_ref,a.currency,coalesce(l.balance,0) balance_micros,
 coalesce(h.held,0) held_micros,coalesce(l.balance,0)-coalesce(h.held,0) available_micros
FROM billing_accounts a
LEFT JOIN LATERAL (SELECT sum(amount_micros) balance FROM billing_entries WHERE account_id=a.id) l ON true
LEFT JOIN LATERAL (SELECT sum(r.cap_micros) held FROM billing_orders o JOIN billing_rates r ON r.id=o.rate_id
 WHERE o.account_id=a.id AND o.status IN ('reserved','running','delivered')) h ON true;

-- A missing partition only matters when that partition has a different price.
CREATE FUNCTION billing_amount(r billing_rates, state text, t bigint, i bigint, o bigint, images integer, cached numeric)
RETURNS numeric LANGUAGE sql IMMUTABLE AS $$
 SELECT CASE
 WHEN state IN ('inconsistent','unresolved') OR t<i OR t<o THEN NULL
 WHEN r.unit='image' THEN images::numeric*r.unit_micros
 WHEN state<>'measured' THEN NULL
 WHEN r.unit='total_tokens' THEN div(t::numeric*r.unit_micros+500000,1000000)
 WHEN r.unit='split_tokens' AND i IS NOT NULL AND o IS NOT NULL THEN
  CASE WHEN r.cached_micros IS NULL OR r.cached_micros=r.input_micros THEN
    div(i::numeric*r.input_micros+o::numeric*r.output_micros+500000,1000000)
  WHEN cached BETWEEN 0 AND i THEN
    div((i-cached)*r.input_micros+cached*r.cached_micros+o::numeric*r.output_micros+500000,1000000)
  END
 END
$$;
CREATE FUNCTION billing_cached_tokens(u jsonb) RETURNS numeric LANGUAGE sql IMMUTABLE AS $$
 SELECT CASE WHEN v ~ '^[0-9]+$' THEN v::numeric END FROM (SELECT coalesce(u#>>'{input_tokens_details,cached_tokens}',u#>>'{prompt_tokens_details,cached_tokens}',u->>'prompt_cache_hit_tokens') v) counts
$$;
CREATE VIEW billing_order_details AS
SELECT o.*, r.currency,r.cap_micros FROM billing_orders o JOIN billing_rates r ON r.id=o.rate_id;

-- Only a deliberately dispatched order may own a new billable transport attempt.
CREATE FUNCTION billing_identify_order() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.subject LIKE 'order:%' THEN
  IF NOT EXISTS(SELECT 1 FROM billing_orders o JOIN billing_rates r ON r.id=o.rate_id
     WHERE o.id::text=substring(NEW.subject FROM 7) AND NEW.purpose='image' AND NEW.operation='images/generations'
     AND NEW.image_size=o.request->>'size'
     AND r.endpoint=NEW.endpoint AND r.provider_account=NEW.provider_account AND r.model=NEW.requested_model)
  THEN RAISE EXCEPTION 'invalid billing attribution'; END IF;
  NEW.cost_owner := 'customer';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER billing_call_owner BEFORE INSERT ON model_calls FOR EACH ROW EXECUTE FUNCTION billing_identify_order();

-- Prefer complete supplier evidence without counting a replay twice. A cache breakdown
-- matters financially even when two observations agree on their total tokens.
CREATE VIEW billing_receipts AS
 WITH merged AS (
  SELECT endpoint,provider_account,receipt_key,max(id::text) FILTER (WHERE receipt_rank=1)::uuid call_id,
    min(receipt_started_at) started_at,coalesce(max(resolved_model),max(requested_model)) model,
    min(subject) subject, min(subject)<>max(subject) multiple_subjects,
    bool_or(outcome='ok') successful,bool_or(outcome IN ('started','unknown')) unfinished,
    bool_or(meter_state='inconsistent') OR
    min(billing_cached_tokens(usage)) IS DISTINCT FROM max(billing_cached_tokens(usage)) AS conflicting,
    max(total_tokens) total_tokens,max(input_tokens) input_tokens,max(output_tokens) output_tokens,
    max(images) images,max(billing_cached_tokens(usage)) cached
  FROM model_usage_receipts GROUP BY endpoint,provider_account,receipt_key
 )
 SELECT m.*,CASE WHEN conflicting OR cached>input_tokens OR total_tokens<input_tokens OR total_tokens<output_tokens
    OR total_tokens::numeric<>input_tokens::numeric+output_tokens THEN 'inconsistent'
   WHEN total_tokens IS NOT NULL THEN 'measured'
   WHEN input_tokens IS NOT NULL OR output_tokens IS NOT NULL THEN 'partial'
   WHEN unfinished THEN 'unresolved' ELSE 'unreported' END meter_state FROM merged m;

CREATE FUNCTION billing_daily_costs(d date,cur text)
RETURNS TABLE(call_id uuid,endpoint text,provider_account text,receipt_key text,model text,subject text,meter_state text,rate_id uuid,cost_micros numeric,multiple_subjects boolean)
LANGUAGE sql STABLE AS $$
 SELECT v.call_id,v.endpoint,v.provider_account,v.receipt_key,v.model,v.subject,v.meter_state,r.id,
  billing_amount(r,v.meter_state,v.total_tokens,v.input_tokens,v.output_tokens,v.images,v.cached),v.multiple_subjects
 FROM billing_receipts v LEFT JOIN LATERAL (
 SELECT p.* FROM billing_rates p WHERE p.kind='cost' AND p.endpoint=v.endpoint AND p.provider_account=v.provider_account
  AND p.model=v.model AND p.currency=cur AND p.effective_at<=v.started_at ORDER BY effective_at DESC LIMIT 1
 ) r ON true
 WHERE v.started_at >= (d::timestamp AT TIME ZONE 'UTC') AND v.started_at < ((d+1)::timestamp AT TIME ZONE 'UTC')
$$;

-- name: 031.sql
ALTER TABLE billing_orders ADD COLUMN automatic boolean NOT NULL DEFAULT false;
CREATE INDEX billing_queued_previews ON billing_orders(seq) WHERE automatic AND status='reserved';

CREATE TABLE billing_reader_sessions (
 token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash)=32),
 account_id uuid NOT NULL REFERENCES billing_accounts,
 expires_at timestamptz NOT NULL DEFAULT now()+interval '12 hours'
);
CREATE INDEX billing_reader_session_expiry ON billing_reader_sessions(expires_at);

CREATE TABLE billing_preview_workers (
 endpoint text NOT NULL, provider_account text NOT NULL, model text NOT NULL,
 available_until timestamptz NOT NULL,
 PRIMARY KEY(endpoint,provider_account,model)
);

-- name: 032.sql
CREATE TABLE billing_alipay_topups (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 account_id uuid NOT NULL REFERENCES billing_accounts, client_key uuid NOT NULL,
 amount_fen integer NOT NULL CHECK (amount_fen BETWEEN 1 AND 1000000),
 app_id text NOT NULL, seller_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '30 minutes',
 trade_no text UNIQUE, paid_at timestamptz,
 closed boolean NOT NULL DEFAULT false,
 entry_id bigint UNIQUE REFERENCES billing_entries,
 UNIQUE(account_id,client_key),
 CHECK ((paid_at IS NULL)=(entry_id IS NULL) AND (trade_no IS NULL)=(entry_id IS NULL))
);
CREATE INDEX billing_topups_reader ON billing_alipay_topups(account_id,created_at DESC);

-- name: 033.sql
CREATE TABLE editor_tasks (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), request_id uuid NOT NULL UNIQUE, world text NOT NULL, chapter text NOT NULL,
 instruction text NOT NULL, author text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 status text NOT NULL DEFAULT 'evidence' CHECK(status IN ('evidence','drafting','checking','review','published','failed','conflict')),
 round integer NOT NULL DEFAULT 1, attempt integer NOT NULL DEFAULT 1, retries integer NOT NULL DEFAULT 0,
 lease uuid, due timestamptz NOT NULL DEFAULT now(), base_head uuid, base jsonb NOT NULL, sources jsonb NOT NULL DEFAULT '[]',
 draft jsonb NOT NULL DEFAULT '[]', feedback text NOT NULL DEFAULT '', issue text NOT NULL DEFAULT '',
 reviewer text, published_at timestamptz
);
CREATE INDEX editor_queue ON editor_tasks(due) WHERE status IN ('evidence','drafting','checking');
CREATE TABLE editor_events(id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,task uuid NOT NULL REFERENCES editor_tasks, at timestamptz NOT NULL DEFAULT now(), round integer NOT NULL,attempt integer NOT NULL,stage text NOT NULL,note text NOT NULL,detail jsonb);
CREATE INDEX editor_task_history ON editor_events(task,id);
CREATE TABLE editor_heads(world text NOT NULL,chapter text NOT NULL, task uuid NOT NULL REFERENCES editor_tasks, patch jsonb NOT NULL, PRIMARY KEY(world,chapter));

CREATE FUNCTION editor_raw_chapter(text,text) RETURNS jsonb LANGUAGE sql STABLE AS $$
SELECT jsonb_build_object('work',w.card,'chapter',c.card,'accepted_art',(SELECT jsonb_agg(p-ARRAY['sha','slot','round','variant','phone','phone_sha'] ORDER BY ord) FROM visible_reader_scopes s CROSS JOIN jsonb_array_elements(s.picks) WITH ORDINALITY e(p,ord) WHERE s.world_slug=w.slug AND s.v=c.v AND c.episode BETWEEN s.first_episode AND s.last_episode AND p->>'chapter'=c.slug),
    'content_kind',CASE w.origin WHEN 'original' THEN 'original' ELSE 'editorial_guide' END,
    'research',(SELECT jsonb_build_object('run_id',r.run_id,'paragraphs',r.paragraphs)
      FROM chapter_research r WHERE (r.v,r.slug)=(c.v,c.slug) ORDER BY r.run_id LIMIT 1),
    'reader_passages',coalesce((SELECT s.reader_copy->c.slug FROM visible_reader_scopes s WHERE s.v=c.v AND s.world_slug=w.slug AND c.episode BETWEEN s.first_episode AND s.last_episode), (SELECT jsonb_agg(jsonb_build_object('key',coalesce(p->>'block','episode-'||ord::text),'text',p->>'text',
      'anchors',coalesce((SELECT jsonb_agg('passage-'||s.slug ORDER BY s.position) FROM work_scenes s
        WHERE (s.v,s.chapter,s.block)=(c.v,c.slug,p->>'block')),'[]')) ORDER BY ord)
      FROM edition_reader_chapters i JOIN chapter_research cr ON (cr.v,cr.slug)=(c.v,c.slug)
      JOIN research_runs rr ON rr.id=cr.run_id AND rr.documents @> i.sources
      CROSS JOIN jsonb_array_elements(i.passages) WITH ORDINALITY a(p,ord)
      WHERE (i.world_slug,i.chapter)=(w.slug,c.slug))),
    'next_chapter',coalesce((SELECT n.card FROM published_chapters n WHERE n.v=c.v AND n.position>c.position ORDER BY n.position LIMIT 1),(SELECT e.chapter FROM published_extensions e WHERE e.v=c.v AND e.first_episode=c.episode+1)),
    'blocks',(SELECT jsonb_agg(jsonb_build_object('key',b.key,'text',b.text,'anchors',coalesce((SELECT jsonb_agg('passage-'||s.slug ORDER BY s.position) FROM work_scenes s
      WHERE (s.v,s.chapter,s.block)=(b.v,b.chapter,b.key)),'[]')) ORDER BY b.position) FROM chapter_blocks b WHERE (b.v,b.chapter)=(c.v,c.slug)),
    'scenes',coalesce((SELECT jsonb_agg(s.card ORDER BY s.position) FROM published_scenes s WHERE (s.v,s.chapter)=(c.v,c.slug)),'[]'),
    'attribution', (SELECT jsonb_agg(jsonb_build_object('uri',e.uri,'locator',e.locator) ORDER BY e.key) FROM world_evidence e WHERE e.v=c.v AND
      (e.key=c.evidence OR EXISTS(SELECT FROM chapter_appearances a WHERE (a.v,a.chapter,a.evidence)=(e.v,c.slug,e.key)))))
    FROM published_worlds w JOIN published_chapters c ON w.version_id=c.v WHERE w.slug=$1 AND c.slug=$2
    UNION ALL SELECT jsonb_build_object('work',w.card,'chapter',e.chapter,'content_kind','editorial_guide',
      'accepted_art',(SELECT jsonb_agg(p-ARRAY['sha','slot','round','variant','phone','phone_sha']) FROM jsonb_array_elements(e.picks) p),
      'research',jsonb_build_object('run_id',r.id,'paragraphs',r.claims),'reader_passages',e.reader_copy->e.slug,
      'blocks',e.reader_copy->e.slug,'scenes',jsonb_build_array(e.scene),'next_chapter',(SELECT x.chapter FROM published_extensions x WHERE x.v=e.v AND x.first_episode=e.last_episode+1),'attribution','[]'::jsonb)
    FROM published_extensions e JOIN published_worlds w ON w.version_id=e.v JOIN research_runs r ON r.id=e.research_run WHERE w.slug=$1 AND e.slug=$2
$$;
CREATE FUNCTION editor_raw_scene(text,text) RETURNS jsonb LANGUAGE sql STABLE AS $$
SELECT jsonb_build_object('work',w.card,'scene',s.card||jsonb_build_object('text',s.text),'chapter',c.card,'setting',s.place,
    'scene_cast',coalesce((SELECT jsonb_agg(p.card ORDER BY p.position) FROM scene_appearances a JOIN published_characters p ON (p.v,p.slug)=(a.v,a.person)
     WHERE (a.v,a.scene)=(s.v,s.slug)),'[]'),
    'return_to',jsonb_build_object('block_key',s.block,'href',(c.card->>'href')||'#passage-'||s.slug),
    'attribution',(SELECT jsonb_agg(jsonb_build_object('uri',e.uri,'locator',e.locator) ORDER BY e.key) FROM world_evidence e WHERE e.v=s.v AND
      (e.key=s.evidence OR EXISTS(SELECT FROM scene_appearances a WHERE (a.v,a.scene,a.evidence)=(e.v,s.slug,e.key)))))
    FROM published_worlds w JOIN published_scenes s ON s.v=w.version_id JOIN published_chapters c ON (c.v,c.slug)=(s.v,s.chapter) WHERE w.slug=$1 AND s.slug=$2
    UNION ALL SELECT jsonb_build_object('work',w.card,'chapter',e.chapter,'setting',e.place,'scene',e.scene||jsonb_build_object('text',e.reader_copy->e.slug->coalesce((plan.spec->>'sceneBeat')::int-1,1)->>'text'),
      'scene_cast',coalesce((SELECT jsonb_agg(card ORDER BY ord) FROM extension_cast c WHERE c.topic=e.topic AND ((plan.id IS NULL AND c.slug='spike') OR (plan.id IS NOT NULL AND strpos(plan.spec->'art'->>'brief',c.card->>'name')>0))),'[]'::jsonb),
      'return_to',jsonb_build_object('block_key',e.picks->0->>'block','href',(e.chapter->>'href')||'#passage-'||e.scene_slug),'attribution','[]'::jsonb)
    FROM published_extensions e JOIN published_worlds w ON w.version_id=e.v LEFT JOIN episode_plans plan USING(topic) WHERE w.slug=$1 AND e.scene_slug=$2
$$;

-- Only substitute reviewed text whose original still matches. IDs, order, artwork and links always come from the current reader projection.
CREATE FUNCTION editor_render(p jsonb, patch jsonb) RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE field text; entry jsonb; blocks jsonb; key text;
BEGIN
 IF p IS NULL OR patch IS NULL OR patch='{}' THEN RETURN p; END IF;
 IF p ? 'return_to' THEN
   entry:=patch->('scene/'||(p#>>'{scene,slug}'));
   IF entry->>'original'=p#>>'{scene,text}' THEN RETURN jsonb_set(p,'{scene,text}',entry->'text'); END IF;
   RETURN p;
 END IF;
 field:=CASE WHEN jsonb_typeof(p->'reader_passages')='array' THEN 'reader_passages' ELSE 'blocks' END;
 SELECT jsonb_agg(CASE WHEN b->>'text'=patch->('block/'||(b->>'key'))->>'original'
   THEN jsonb_set(b,'{text}',patch->('block/'||(b->>'key'))->'text') ELSE b END ORDER BY n) INTO blocks
 FROM jsonb_array_elements(p->field) WITH ORDINALITY a(b,n);
 IF blocks IS NOT NULL THEN p:=jsonb_set(p,ARRAY[field],blocks); END IF;
 RETURN p;
END $$;
CREATE FUNCTION editor_public_page(p jsonb,w text) RETURNS jsonb LANGUAGE sql STABLE AS $$
 SELECT editor_render(p,(SELECT patch FROM editor_heads WHERE world=w AND chapter=p#>>'{chapter,slug}'))
$$;
CREATE FUNCTION editor_snapshot(w text,c text) RETURNS jsonb LANGUAGE sql STABLE AS $$
 WITH chapter AS (SELECT editor_public_page(editor_raw_chapter(w,c),w) page)
 SELECT jsonb_build_object('chapter',page,'scenes',coalesce((SELECT jsonb_object_agg(s->>'slug',editor_public_page(editor_raw_scene(w,s->>'slug'),w))
 FROM jsonb_array_elements(page->'scenes') s),'{}'::jsonb)) FROM chapter WHERE page IS NOT NULL
$$;
CREATE TABLE editor_sessions(hash bytea PRIMARY KEY,operator text NOT NULL,expires_at timestamptz NOT NULL DEFAULT now()+interval '8 hours');

-- name: 034.sql
-- Chapters have their own publication head. Profiles and setting share one head per work.
ALTER TABLE editor_tasks ADD COLUMN kind text NOT NULL DEFAULT 'chapter' CHECK (kind IN ('chapter','profiles'));
ALTER TABLE editor_heads DROP CONSTRAINT editor_heads_pkey, ALTER COLUMN chapter DROP NOT NULL;
ALTER TABLE editor_heads ADD UNIQUE NULLS NOT DISTINCT (world,chapter);

CREATE FUNCTION editor_raw_overview(text) RETURNS jsonb LANGUAGE sql STABLE AS $$
SELECT jsonb_build_object('work',w.card,
    'completed_scope',(SELECT jsonb_build_object('first',min(s.first_episode),'last',max(s.last_episode),
      'picks',jsonb_agg(p-ARRAY['sha','slot','round','variant','phone','phone_sha'] ORDER BY s.first_episode,ord))
      FROM visible_reader_scopes s CROSS JOIN jsonb_array_elements(s.picks) WITH ORDINALITY e(p,ord)
      WHERE s.v=w.version_id AND s.world_slug=w.slug HAVING count(*)>0),
    'reader_intro',coalesce((SELECT jsonb_build_object('premise',i.premise,'setting',i.setting) FROM edition_reader_intros i WHERE i.world_slug=w.slug),
      (SELECT jsonb_build_object('premise',w.intro,'setting',e.reader_copy->e.slug->((p.spec->'setting'->>1)::int-1)->>'text') FROM published_extensions e JOIN episode_plans p USING(topic) WHERE e.v=w.version_id ORDER BY e.first_episode LIMIT 1)),
    'characters',coalesce((SELECT jsonb_agg(card ORDER BY position) FROM published_characters WHERE v=w.version_id),'[]') || coalesce((SELECT jsonb_agg(card ORDER BY slug) FROM extension_people WHERE v=w.version_id),'[]'),
    'storylines',coalesce((SELECT jsonb_agg(jsonb_build_object('slug',l.slug,'title',l.title,'intro',l.intro,'kind','editorial_guide',
      'chapters',coalesce((SELECT jsonb_agg(c.card ORDER BY m.position) FROM work_storyline_chapters m JOIN published_chapters c ON (c.v,c.slug)=(m.v,m.chapter)
          WHERE (m.v,m.line)=(l.v,l.slug)),'[]')) ORDER BY l.position) FROM work_storylines l WHERE l.v=w.version_id),'[]') || coalesce((SELECT jsonb_agg(jsonb_build_object('slug',e.slug,'title',e.scene_name,'intro',e.intro,'kind','editorial_guide','chapters',jsonb_build_array(e.chapter)) ORDER BY e.first_episode) FROM published_extensions e WHERE e.v=w.version_id),'[]'),
    'scenes',coalesce((SELECT jsonb_agg(s.card ORDER BY c.position,s.position) FROM published_scenes s JOIN work_chapters c ON (c.v,c.slug)=(s.v,s.chapter) WHERE s.v=w.version_id),'[]') || coalesce((SELECT jsonb_agg(e.scene ORDER BY e.first_episode) FROM published_extensions e WHERE e.v=w.version_id),'[]'))
    FROM published_worlds w WHERE w.slug=$1
$$;

CREATE FUNCTION editor_raw_person(text,text) RETURNS jsonb LANGUAGE sql STABLE AS $$
SELECT jsonb_build_object('work',w.card,'character',p.card,'intro',coalesce((SELECT i.people->>p.slug FROM edition_reader_intros i WHERE i.world_slug=w.slug), CASE WHEN w.slug='tidepost' AND p.slug='mio' THEN '阿澪熟悉港口的每一盏航灯。她的制服口袋里总装着潮汐表和一枚铜哨，偶尔还有一封舍不得退回的信。' END),'scene_previews',coalesce((SELECT jsonb_agg(s.card ORDER BY s.slug) FROM scene_appearances a JOIN published_scenes s ON (a.v,a.scene)=(s.v,s.slug) WHERE (a.v,a.person)=(p.v,p.slug)),'[]') || coalesce((SELECT jsonb_agg(e.scene) FROM published_extensions e WHERE e.v=p.v AND p.slug='spike'),'[]'),'appearances',coalesce((
    SELECT jsonb_agg(c.card ORDER BY c.position) FROM chapter_appearances a JOIN published_chapters c ON (c.v,c.slug)=(a.v,a.chapter)
    WHERE (a.v,a.person)=(p.v,p.slug)),'[]') || coalesce((SELECT jsonb_agg(e.chapter ORDER BY e.first_episode) FROM published_extensions e WHERE e.v=p.v AND e.people @> jsonb_build_array(jsonb_build_object('slug',p.slug))),'[]')) FROM published_worlds w JOIN published_characters p ON p.v=w.version_id WHERE w.slug=$1 AND p.slug=$2
    UNION ALL SELECT jsonb_build_object('work',w.card,'character',p.card,'intro',p.intro,
      'scene_previews',coalesce((SELECT jsonb_agg(x.scene ORDER BY x.first_episode) FROM published_extensions x JOIN episode_plans plan USING(topic) WHERE x.v=p.v AND x.people @> jsonb_build_array(jsonb_build_object('slug',p.slug)) AND strpos(plan.spec->'art'->>'brief',p.card->>'name')>0),'[]'),
      'appearances',(SELECT jsonb_agg(x.chapter ORDER BY x.first_episode) FROM published_extensions x WHERE x.v=p.v AND x.people @> jsonb_build_array(jsonb_build_object('slug',p.slug))))
    FROM extension_people p JOIN published_extensions e ON e.id=p.id JOIN published_worlds w ON w.version_id=p.v WHERE w.slug=$1 AND p.slug=$2
$$;


CREATE FUNCTION editor_profile_text(p jsonb,k text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
 SELECT coalesce(CASE WHEN k='world/setting' THEN coalesce(p#>>'{reader_intro,setting}',p->>'world_setting') ELSE p->>'intro' END,'')
$$;

CREATE FUNCTION editor_render_profile(p jsonb,k text,patch jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
 SELECT CASE WHEN patch->k->>'original'=editor_profile_text(p,k) THEN
   jsonb_set(p,CASE WHEN k<>'world/setting' THEN ARRAY['intro']
      WHEN jsonb_typeof(p->'reader_intro')='object' THEN ARRAY['reader_intro','setting']
      ELSE ARRAY['world_setting'] END,patch->k->'text') ELSE p END
$$;

CREATE FUNCTION editor_public_profile(p jsonb,w text,k text) RETURNS jsonb LANGUAGE sql STABLE AS $$
 SELECT editor_render_profile(p,k,(SELECT patch FROM editor_heads WHERE world=w AND chapter IS NULL))
$$;

-- Determine editable people from the published appearance relation of the selected chapter.
CREATE FUNCTION editor_task_snapshot(w text,c text,k text) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
BEGIN
 IF k='chapter' THEN RETURN editor_snapshot(w,c); END IF;
 RETURN (
   WITH chapter AS (SELECT editor_raw_chapter(w,c) page),
   overview AS (SELECT editor_raw_overview(w) page),
   pages(field,page) AS (
     SELECT 'world/setting',page FROM overview WHERE page IS NOT NULL
     UNION ALL
     SELECT 'person/'||(person->>'slug'),p FROM overview o
       CROSS JOIN jsonb_array_elements(o.page->'characters') person
       CROSS JOIN LATERAL editor_raw_person(w,person->>'slug') p
       WHERE p->'appearances' @> jsonb_build_array(jsonb_build_object('slug',c))
   ), visible AS (SELECT field,editor_public_profile(page,w,field) page FROM pages)
   SELECT jsonb_build_object('profile_pages',jsonb_object_agg(field,page),'fields',jsonb_object_agg(field,editor_profile_text(page,field)),
     'research_run',(SELECT page#>>'{research,run_id}' FROM chapter))
   FROM visible HAVING count(*)>0 AND (SELECT page IS NOT NULL FROM chapter)
 );
END $$;

CREATE FUNCTION editor_profile_preview(base jsonb,draft jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
 WITH patch AS (SELECT jsonb_object_agg(d->>'key',jsonb_build_object('text',d->'text','original',base->'fields'->(d->>'key'))) edits
   FROM jsonb_array_elements(draft) d)
 SELECT jsonb_build_object('profile_pages',jsonb_object_agg(key,editor_render_profile(value,key,edits))) FROM jsonb_each(base->'profile_pages'),patch
$$;

-- name: 035.sql
-- Each operator-authorized resume gets a fresh Pi task identity. The scheduler
-- still makes at most one automatic retry per initial provider failure.
ALTER TABLE media_candidates DROP CONSTRAINT media_candidates_attempt_check,
  ADD CHECK (attempt >= 1);
ALTER TABLE media_steps DROP CONSTRAINT media_steps_attempt_check,
  ADD CHECK (attempt >= 1);

-- name: 036.sql
CREATE TABLE reader_identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    import_choice text CHECK (import_choice IN ('merge','skip'))
);
CREATE TABLE reader_email_codes (
    email text PRIMARY KEY,
    digest bytea NOT NULL,
    issued_at timestamptz NOT NULL DEFAULT now(),
    attempts int NOT NULL DEFAULT 0
);
CREATE INDEX reader_codes_issued ON reader_email_codes(issued_at);
CREATE TABLE reader_sessions (
    digest bytea PRIMARY KEY,
    reader_id uuid NOT NULL REFERENCES reader_identities ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);
CREATE INDEX reader_session_expiry ON reader_sessions(expires_at);
CREATE TABLE reader_bookmarks (
    reader_id uuid NOT NULL REFERENCES reader_identities ON DELETE CASCADE,
    href text NOT NULL,
    title text NOT NULL,
    work text NOT NULL,
    recorded_at bigint NOT NULL,
    PRIMARY KEY(reader_id, href)
);
CREATE TABLE reader_positions (
    reader_id uuid NOT NULL REFERENCES reader_identities ON DELETE CASCADE,
    href text NOT NULL,
    title text NOT NULL,
    work text NOT NULL,
    percent int NOT NULL CHECK(percent BETWEEN 0 AND 100),
    anchor text NOT NULL,
    excerpt text NOT NULL,
    recorded_at bigint NOT NULL,
    PRIMARY KEY(reader_id, href)
);

-- name: 037.sql
-- Existing email accounts keep their reader ID, sessions and reading history.
ALTER TABLE reader_identities ALTER COLUMN email DROP NOT NULL;
CREATE TABLE reader_wechat (
  reader_id uuid PRIMARY KEY REFERENCES reader_identities ON DELETE CASCADE,
  app_id text NOT NULL,
  open_id text NOT NULL,
  UNIQUE(app_id, open_id)
);
CREATE TABLE reader_wechat_flows (
  state text PRIMARY KEY,
  browser_hash bytea NOT NULL,
  bind_session bytea REFERENCES reader_sessions(digest) ON DELETE CASCADE,
  return_to text NOT NULL,
  expires_at timestamptz NOT NULL
);
CREATE INDEX reader_wechat_flows_expiry ON reader_wechat_flows(expires_at);

-- name: 038.sql
CREATE TABLE reader_plans (
    id text PRIMARY KEY CHECK (id IN ('free','plus','pro')),
    shelf_limit integer NOT NULL CHECK (shelf_limit>0)
);
INSERT INTO reader_plans VALUES ('free',100),('plus',500),('pro',2000);
ALTER TABLE reader_identities
    ADD COLUMN plan text NOT NULL DEFAULT 'free' REFERENCES reader_plans,
    ADD COLUMN plan_until timestamptz;
CREATE TABLE reader_plan_changes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reader_id uuid NOT NULL REFERENCES reader_identities ON DELETE CASCADE,
    plan text NOT NULL REFERENCES reader_plans,
    valid_until timestamptz,
    operator text NOT NULL,
    changed_at timestamptz NOT NULL DEFAULT now()
);
CREATE VIEW reader_memberships AS
    SELECT r.id AS reader_id,p.id AS plan,p.shelf_limit,
           CASE WHEN p.id<>'free' THEN r.plan_until END AS valid_until
    FROM reader_identities r
    JOIN reader_plans p ON p.id=CASE WHEN r.plan_until<=now() THEN 'free' ELSE r.plan END;

-- name: 039.sql
ALTER TABLE reader_plans ADD COLUMN price_fen integer NOT NULL DEFAULT 0 CHECK(price_fen>=0),
    ADD COLUMN credits integer NOT NULL DEFAULT 0 CHECK(credits>=0);
UPDATE reader_plans p SET price_fen=v.fen,credits=v.credits FROM
    (VALUES ('free',0,10),('plus',2000,125),('pro',9900,625)) AS v(id,fen,credits) WHERE p.id=v.id;

CREATE TABLE reader_plan_orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reader_id uuid NOT NULL REFERENCES reader_identities,
    plan text NOT NULL REFERENCES reader_plans CHECK(plan<>'free'),
    amount_fen integer NOT NULL CHECK(amount_fen>0),
    credits integer NOT NULL CHECK(credits>0),
    app_id text NOT NULL,seller_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL DEFAULT now()+interval '15 minutes',
    trade_no text UNIQUE, closed boolean NOT NULL DEFAULT false,
    paid_at timestamptz, starts_at timestamptz, ends_at timestamptz,
    refunded_at timestamptz,
    CHECK ((paid_at IS NULL)=(trade_no IS NULL) AND (starts_at IS NULL)=(paid_at IS NULL)
      AND (ends_at IS NULL)=(paid_at IS NULL) AND ends_at>starts_at)
);
CREATE INDEX reader_plan_order_history ON reader_plan_orders(reader_id,created_at DESC);
CREATE TABLE reader_credit_usage (
    reader_id uuid NOT NULL REFERENCES reader_identities,
    operation_id uuid NOT NULL UNIQUE,
    period_start timestamptz NOT NULL,
    microcredits bigint NOT NULL CHECK(microcredits>0),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(reader_id,operation_id)
);
CREATE INDEX reader_credit_period ON reader_credit_usage(reader_id,period_start);
CREATE TRIGGER reader_credit_usage_immutable BEFORE UPDATE OR DELETE ON reader_credit_usage
    FOR EACH ROW EXECUTE FUNCTION billing_append_only();

-- A paid period takes precedence over an operational grant. Both use the same reader ID.
CREATE OR REPLACE VIEW reader_memberships AS
 SELECT r.id reader_id,p.id plan,p.shelf_limit,
    CASE WHEN o.id IS NOT NULL THEN o.ends_at WHEN p.id<>'free' THEN r.plan_until END valid_until,
    coalesce(o.starts_at,date_trunc('month',now(),'Asia/Shanghai')) period_start,
    coalesce(o.ends_at,((date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')+interval '1 month') AT TIME ZONE 'Asia/Shanghai')) period_end,
    coalesce(o.credits,p.credits) credits
 FROM reader_identities r
 LEFT JOIN LATERAL (SELECT * FROM reader_plan_orders WHERE reader_id=r.id AND refunded_at IS NULL
    AND starts_at<=now() AND ends_at>now() ORDER BY starts_at DESC LIMIT 1) o ON true
 JOIN reader_plans p ON p.id=coalesce(o.plan,CASE WHEN r.plan_until<=now() THEN 'free' ELSE r.plan END);

-- name: 040.sql
ALTER TABLE reader_wechat DROP CONSTRAINT reader_wechat_pkey;
ALTER TABLE reader_wechat ADD PRIMARY KEY (reader_id,app_id);
CREATE TABLE reader_wechat_union (
    subject text PRIMARY KEY,
    reader_id uuid NOT NULL UNIQUE REFERENCES reader_identities ON DELETE CASCADE
);
CREATE TABLE reader_apple (
    subject text PRIMARY KEY,
    reader_id uuid NOT NULL UNIQUE REFERENCES reader_identities ON DELETE CASCADE
);
CREATE TABLE reader_native_challenges (
    nonce text PRIMARY KEY, expires_at timestamptz NOT NULL DEFAULT now()+interval '5 minutes'
);
CREATE INDEX reader_native_challenge_expiry ON reader_native_challenges(expires_at);

CREATE TABLE reader_store_purchases (
    transaction_id text PRIMARY KEY,
    reader_id uuid NOT NULL REFERENCES reader_identities,
    environment text NOT NULL,
    bundle_id text NOT NULL, product_id text NOT NULL,
    plan text NOT NULL REFERENCES reader_plans CHECK(plan <> 'free'),
    credits integer NOT NULL CHECK(credits>0),
    purchased_at timestamptz NOT NULL,
    starts_at timestamptz NOT NULL, ends_at timestamptz NOT NULL CHECK(ends_at>starts_at),
    revoked_at timestamptz, signed_at timestamptz NOT NULL,
    currency text, price_milli bigint
);
CREATE INDEX reader_store_periods ON reader_store_purchases(reader_id,ends_at);
CREATE VIEW reader_paid_periods AS
 SELECT reader_id,plan,credits,starts_at,ends_at FROM reader_plan_orders WHERE paid_at IS NOT NULL AND refunded_at IS NULL
 UNION ALL SELECT reader_id,plan,credits,starts_at,ends_at FROM reader_store_purchases WHERE revoked_at IS NULL;
CREATE OR REPLACE VIEW reader_memberships AS
 SELECT r.id reader_id,p.id plan,p.shelf_limit,
    CASE WHEN o.plan IS NOT NULL THEN o.ends_at WHEN p.id<>'free' THEN r.plan_until END valid_until,
    coalesce(o.starts_at,date_trunc('month',now(),'Asia/Shanghai')) period_start,
    coalesce(o.ends_at,((date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')+interval '1 month') AT TIME ZONE 'Asia/Shanghai')) period_end,
    coalesce(o.credits,p.credits) credits
 FROM reader_identities r
 LEFT JOIN LATERAL (SELECT * FROM reader_paid_periods WHERE reader_id=r.id AND starts_at<=now() AND ends_at>now()
    ORDER BY starts_at DESC LIMIT 1) o ON true
 JOIN reader_plans p ON p.id=coalesce(o.plan,CASE WHEN r.plan_until<=now() THEN 'free' ELSE r.plan END);

-- name: 041.sql
-- Order refund and reversal events separately from ordinary transaction refreshes.
ALTER TABLE reader_store_purchases ADD COLUMN refund_event_at timestamptz;
UPDATE reader_store_purchases SET refund_event_at=signed_at WHERE revoked_at IS NOT NULL;

-- name: 042.sql
CREATE TABLE billing_store_topups (
    transaction_id text PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES billing_accounts,
    environment text NOT NULL, bundle_id text NOT NULL,
    credited boolean NOT NULL DEFAULT false,
    refund_event_at timestamptz,
    purchased_at timestamptz NOT NULL,
    currency text, price_milli bigint
);
ALTER TABLE billing_entries DROP CONSTRAINT billing_entries_kind_check, DROP CONSTRAINT billing_entries_check;
ALTER TABLE billing_entries ADD CONSTRAINT billing_entries_check CHECK (
    (kind IN ('credit','store_credit') AND order_id IS NULL AND amount_micros>0)
 OR (kind='store_refund' AND order_id IS NULL AND amount_micros<0)
 OR (kind='charge' AND order_id IS NOT NULL AND amount_micros<=0)
 OR (kind='refund' AND order_id IS NOT NULL AND amount_micros>=0)
);

-- name: 043.sql
CREATE TABLE episode_inventory_snapshots (
    checked_at timestamptz PRIMARY KEY,
    works jsonb NOT NULL CHECK (jsonb_typeof(works)='array' AND jsonb_array_length(works)>0)
);

-- name: 044.sql
CREATE TABLE episode_source_captures (
 topic text PRIMARY KEY REFERENCES episode_plans(topic),
 spec jsonb NOT NULL,
 documents jsonb NOT NULL CHECK(jsonb_typeof(documents)='array'),
 captured_at timestamptz NOT NULL
);

-- name: 045.sql
-- Each policy is a lifetime production envelope for one edition. Changing its
-- range never refunds an earlier admission or alters an approved chapter.
CREATE TABLE episode_policies (
 world_slug text PRIMARY KEY REFERENCES work_worlds(slug),
 first_episode int NOT NULL CHECK(first_episode>0),
 last_episode int NOT NULL CHECK(last_episode BETWEEN first_episode AND first_episode+999),
 priority int NOT NULL CHECK(priority BETWEEN 0 AND 100),
 max_episodes int NOT NULL CHECK(max_episodes>=0),
 token_budget bigint NOT NULL CHECK(token_budget>=0),
 reserve_tokens int NOT NULL CHECK(reserve_tokens>0)
);
CREATE TABLE episode_admissions (
 topic text PRIMARY KEY REFERENCES episode_plans(topic),
 reserve_tokens int NOT NULL CHECK(reserve_tokens>0),
 admitted_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE episode_capture_attempts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 topic text NOT NULL REFERENCES episode_plans(topic),
 started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 error text
);
CREATE INDEX episode_capture_attempts_topic ON episode_capture_attempts(topic,started_at DESC);
CREATE UNIQUE INDEX episode_capture_active ON episode_capture_attempts(topic) WHERE finished_at IS NULL;

CREATE VIEW episode_admission_usage AS
 SELECT a.topic,a.reserve_tokens,a.admitted_at,p.world_slug,p.number,
 coalesce((SELECT sum(m.total_tokens) FROM model_usage_receipts m WHERE m.receipt_rank=1
 AND m.subject IN (
  SELECT 'research:'||r.id FROM research_runs r WHERE r.topic=a.topic
  UNION ALL SELECT 'media:'||s.slot||':r'||s.round||':'||s.step||':a'||s.attempt
  FROM episode_art_jobs j JOIN media_candidates c ON c.art_job=j.id JOIN media_steps s ON s.slot=c.slot
  WHERE j.world_slug=p.world_slug AND (j.kind='scene' AND j.slug=p.spec->>'sceneSlug'
  OR j.kind='portrait' AND j.spec->>'slug' IN (SELECT x->>'slug' FROM jsonb_array_elements(p.spec->'people') x)
  AND s.started_at>=a.admitted_at AND NOT EXISTS (SELECT FROM episode_admissions older
    JOIN episode_plans op ON op.topic=older.topic WHERE op.world_slug=p.world_slug AND older.admitted_at<a.admitted_at
    AND op.spec->'people' @> jsonb_build_array(jsonb_build_object('slug',j.slug)))))
 ),0)::bigint AS measured_tokens
 FROM episode_admissions a JOIN episode_plans p USING(topic);

CREATE VIEW episode_policy_queue AS
 WITH inventory AS (SELECT checked_at,works FROM episode_inventory_snapshots ORDER BY checked_at DESC LIMIT 1),
 budgets AS (SELECT world_slug,count(*) AS admissions,sum(greatest(reserve_tokens,measured_tokens)) AS committed
 FROM episode_admission_usage GROUP BY world_slug)
 SELECT pol.*,n AS number,p.topic,a.admitted_at,a.measured_tokens,coalesce(b.committed,0) AS committed,
 h.published,cap.started_at AS capture_started,cap.finished_at AS capture_finished,cap.error AS capture_error,
 CASE WHEN h.published IS NOT NULL THEN '五维通过'
 WHEN a.topic IS NOT NULL THEN CASE WHEN cap.error IS NOT NULL THEN '采集失败' WHEN c.topic IS NULL THEN '待采集' ELSE '已采集' END
 WHEN i.checked_at IS NULL OR i.checked_at<now()-interval '48 hours' THEN '等待有效盘点'
 WHEN NOT EXISTS (SELECT FROM jsonb_array_elements(i.works) w WHERE w->>'slug'=pol.world_slug AND w->'missing' @> to_jsonb(ARRAY[n])) THEN '未确认缺集'
 WHEN p.topic IS NULL THEN '缺分集配置'
 WHEN coalesce(b.admissions,0)>=pol.max_episodes THEN '集数预算已满'
 WHEN coalesce(b.committed,0)+pol.reserve_tokens>pol.token_budget THEN 'token 预算不足'
 ELSE '待准入' END AS status
 FROM episode_policies pol CROSS JOIN LATERAL generate_series(pol.first_episode,pol.last_episode)n
 LEFT JOIN inventory i ON true LEFT JOIN budgets b USING(world_slug)
 LEFT JOIN episode_plans p ON p.world_slug=pol.world_slug AND p.number=n
 LEFT JOIN episode_admission_usage a ON a.topic=p.topic LEFT JOIN scope_heads h ON h.topic=p.topic
 LEFT JOIN episode_source_captures c ON c.topic=p.topic
 LEFT JOIN LATERAL (SELECT * FROM episode_capture_attempts e WHERE e.topic=p.topic ORDER BY started_at DESC LIMIT 1)cap ON true;

-- name: 046.sql
CREATE TABLE reader_release_attempts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), topic text NOT NULL,
 acceptance bigint NOT NULL, started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz, target_id bigint, href text, error text,
 FOREIGN KEY (topic,acceptance) REFERENCES scope_acceptances(topic,id),
 CHECK (finished_at IS NULL OR (target_id IS NOT NULL) <> (error IS NOT NULL))
);
CREATE INDEX reader_release_history ON reader_release_attempts(topic,acceptance,started_at DESC);
CREATE UNIQUE INDEX reader_release_once ON reader_release_attempts(topic,acceptance) WHERE target_id IS NOT NULL;

-- name: 047.sql
CREATE TABLE backup_runs (
    job_uid uuid PRIMARY KEY,
    job_name text NOT NULL,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    status text NOT NULL CHECK (status IN ('running','success','failed')),
    archive_bytes bigint CHECK (archive_bytes > 0),
    verified_tables integer CHECK (verified_tables > 0),
    failure_reason text,
    CHECK ((archive_bytes IS NULL) = (verified_tables IS NULL)),
    CHECK (archive_bytes IS NULL OR status = 'success'),
    CHECK ((finished_at IS NULL) = (status = 'running')),
    CHECK ((failure_reason IS NOT NULL) = (status = 'failed'))
);
CREATE INDEX backup_runs_started ON backup_runs(started_at DESC);
CREATE TABLE backup_observer (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    observed_at timestamptz NOT NULL
);

-- name: 048.sql
ALTER TABLE backup_runs ADD COLUMN trigger_kind text NOT NULL DEFAULT 'unknown'
    CHECK (trigger_kind IN ('manual','scheduled','unknown'));

-- name: 049.sql
CREATE TABLE backup_failed_attempts (
    pod_uid uuid PRIMARY KEY,
    job_uid uuid NOT NULL REFERENCES backup_runs(job_uid),
    finished_at timestamptz NOT NULL,
    reason text NOT NULL
);
CREATE INDEX backup_attempts_by_job ON backup_failed_attempts(job_uid, finished_at);

-- name: 050.sql
CREATE TABLE episode_production_requests (
    id uuid PRIMARY KEY,
    world text NOT NULL,
    first_episode integer NOT NULL CHECK (first_episode > 0),
    last_episode integer NOT NULL CHECK (last_episode >= first_episode),
    token_limit bigint NOT NULL CHECK (token_limit > 0),
    consumed bigint NOT NULL DEFAULT 0 CHECK (consumed >= 0),
    reserved bigint NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    paused_reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE episode_production_requests ADD FOREIGN KEY(world) REFERENCES work_worlds(slug),
    ADD COLUMN plan_brief text,
    ADD COLUMN revision integer NOT NULL DEFAULT 1,
    ADD COLUMN stage text NOT NULL DEFAULT 'inventory',
    ADD COLUMN gaps integer[] NOT NULL DEFAULT '{}',
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TABLE episode_plan_messages (
    request_id uuid NOT NULL REFERENCES episode_production_requests(id),
    revision integer NOT NULL,
    instruction text NOT NULL,
    author text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(request_id,revision)
);

CREATE TABLE episode_call_reservations (
    request_id uuid NOT NULL REFERENCES episode_production_requests(id),
    call_id uuid NOT NULL,
    stage text NOT NULL,
    ceiling bigint NOT NULL CHECK (ceiling > 0),
    actual bigint CHECK (actual >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    settled_at timestamptz,
    PRIMARY KEY (request_id, call_id)
);

-- Each physical dispatch, including a retry, needs its own call ID.
CREATE FUNCTION reserve_episode_call(req uuid, call_key uuid, step text, allowance bigint)
RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE budget episode_production_requests%ROWTYPE;
BEGIN
    IF allowance IS NULL OR allowance <= 0 THEN RAISE EXCEPTION 'invalid call allowance'; END IF;
    SELECT * INTO STRICT budget FROM episode_production_requests WHERE id=req FOR UPDATE;
    IF budget.paused_reason IS NOT NULL OR EXISTS (
        SELECT FROM episode_call_reservations WHERE request_id=req AND call_id=call_key
    ) THEN RETURN false; END IF;
    IF budget.token_limit-budget.consumed-budget.reserved < allowance THEN
        -- An in-flight reservation can still return unused capacity.
        IF budget.reserved=0 THEN
            UPDATE episode_production_requests SET paused_reason='insufficient_remaining_tokens' WHERE id=req;
        END IF;
        RETURN false;
    END IF;
    INSERT INTO episode_call_reservations(request_id,call_id,stage,ceiling)
        VALUES(req,call_key,step,allowance);
    UPDATE episode_production_requests SET reserved=reserved+allowance WHERE id=req;
    RETURN true;
END $$;

CREATE FUNCTION settle_episode_call(req uuid, call_key uuid, used_tokens bigint)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE receipt episode_call_reservations%ROWTYPE;
BEGIN
    PERFORM 1 FROM episode_production_requests WHERE id=req FOR UPDATE;
    SELECT * INTO STRICT receipt FROM episode_call_reservations WHERE request_id=req AND call_id=call_key FOR UPDATE;
    IF receipt.settled_at IS NOT NULL THEN
        IF used_tokens IS DISTINCT FROM receipt.actual THEN RAISE EXCEPTION 'conflicting call receipt'; END IF;
        RETURN;
    END IF;
    IF used_tokens IS NULL THEN
        UPDATE episode_production_requests SET paused_reason='missing_usage_receipt' WHERE id=req;
        RETURN;
    END IF;
    UPDATE episode_call_reservations SET actual=used_tokens,settled_at=now() WHERE request_id=req AND call_id=call_key;
    UPDATE episode_production_requests SET consumed=consumed+used_tokens,reserved=reserved-receipt.ceiling,
        paused_reason=CASE WHEN consumed+used_tokens>=token_limit THEN 'token_limit_reached'
                           WHEN used_tokens>receipt.ceiling THEN 'provider_exceeded_allowance'
                           ELSE paused_reason END WHERE id=req;
END $$;

-- name: 051.sql
CREATE TABLE episode_request_specs (
 request_id uuid NOT NULL REFERENCES episode_production_requests(id),
 revision int NOT NULL, world_slug text NOT NULL, number int NOT NULL,
 spec jsonb NOT NULL, documents jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(request_id,revision,number), UNIQUE(world_slug,number)
);
CREATE FUNCTION reserve_episode_subject(subject text,call_key uuid,step text,allowance bigint)
RETURNS text LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE req uuid;
BEGIN
 SELECT DISTINCT s.request_id INTO req FROM episode_request_specs s JOIN episode_plans p
 ON p.world_slug=s.world_slug AND p.number=s.number WHERE EXISTS(
 SELECT FROM research_runs r WHERE r.topic=p.topic AND 'research:'||r.id=subject)
 OR EXISTS(SELECT FROM media_candidates c JOIN episode_art_jobs j ON j.id=c.art_job
 WHERE j.world_slug=s.world_slug AND j.kind='scene' AND j.slug=p.spec->>'sceneSlug'
 AND split_part(subject,':',1)='media' AND split_part(subject,':',2)=c.slot::text);
 IF req IS NULL THEN RETURN 'unmanaged'; END IF;
 IF reserve_episode_call(req,call_key,step,allowance) THEN RETURN req::text; END IF;
 RETURN 'paused';
END $$;
ALTER FUNCTION settle_episode_call(uuid,uuid,bigint) SECURITY DEFINER;
ALTER FUNCTION settle_episode_call(uuid,uuid,bigint) SET search_path=pg_catalog,public;
REVOKE ALL ON FUNCTION settle_episode_call(uuid,uuid,bigint) FROM PUBLIC;
REVOKE ALL ON FUNCTION reserve_episode_subject(text,uuid,text,bigint) FROM PUBLIC;

-- name: 052.sql
CREATE TABLE workbench_accounts(email text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now());

-- name: 053.sql
CREATE TABLE workbench_editor_grants(email text PRIMARY KEY, granted_at timestamptz NOT NULL DEFAULT now());

-- name: ledger
INSERT INTO knowledge_migrations(version) SELECT lpad(i::text,3,'0') || '.sql' FROM generate_series(1,53) i;
COMMIT;
