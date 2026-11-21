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
