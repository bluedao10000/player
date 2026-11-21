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
