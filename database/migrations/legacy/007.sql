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
