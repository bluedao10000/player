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
