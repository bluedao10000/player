WITH accepted AS (SELECT a.* FROM visible_reader_scopes s JOIN scope_acceptances a ON a.id=s.id
 WHERE s.topic=$1 AND s.id=$2 AND a.passed),
picks AS (SELECT (p->>'slot')::int slot,(p->>'round')::int round FROM accepted a, jsonb_array_elements(a.picks)p),
media AS (SELECT c.* FROM media_candidates c JOIN picks USING(slot,round) WHERE c.stage='passed'),
cast_slugs AS (SELECT e.topic,p->>'slug' slug FROM episode_extensions e JOIN accepted a USING(topic),jsonb_array_elements(e.people)p)
SELECT jsonb_build_object(
 'versions',(select jsonb_agg(to_jsonb(w)-'intro') from (select slug,version_id,work_id from work_worlds where slug in (select edition_slug from accepted))w),
 'portraits',(select jsonb_agg(to_jsonb(p) ORDER BY p.item_slug) from (select distinct r.world_slug,r.item_slug,r.image,r.source_sha256 from cast_slugs x join accepted a using(topic) join edition_reader_art r on r.world_slug=a.edition_slug and r.item_slug=x.slug and r.kind='character')p),
 'research_topics',(select jsonb_agg(to_jsonb(t)) from research_topics t join accepted a on a.topic=t.id),
 'research_runs',(select jsonb_agg(to_jsonb(r)) from research_runs r join accepted a on a.research_run=r.id and r.stage='ready'),
 'episode_extensions',(select jsonb_agg(to_jsonb(e)) from episode_extensions e join accepted using(topic)),
 'episode_plans',(select jsonb_agg(to_jsonb(p)) from episode_plans p join accepted using(topic)),
 'episode_art_jobs',(select jsonb_agg(to_jsonb(j)) from episode_art_jobs j join media c on c.art_job=j.id),
 'media_candidates',(select jsonb_agg(to_jsonb(c)) from media c),
 'scope_acceptances',(select jsonb_agg(to_jsonb(a)) from accepted a));
