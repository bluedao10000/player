WITH input AS (SELECT $1::jsonb j), bundle AS (
 SELECT j->'versions'->0 source_edition,j->'research_topics'->0 topic,j->'research_runs'->0 run,
 j->'episode_plans'->0 plan,j->'episode_extensions'->0 chapter,j->'episode_art_jobs'->0 art,
 j->'media_candidates'->0 media,j->'scope_acceptances'->0 final,j->'portraits' portraits
 FROM input
)
SELECT EXISTS (
 SELECT FROM bundle JOIN published_worlds w ON w.slug=$2 AND w.slug=source_edition->>'slug'
 AND w.work_id::text=source_edition->>'work_id' AND w.version_id=(source_edition->>'version_id')::bigint
 WHERE topic->>'id'=$6 AND topic->>'world_slug'=w.slug
 AND (topic->>'first_episode')::int=$3 AND (topic->>'last_episode')::int=$3
 AND run->>'topic'=$6 AND run->>'id'=final->>'research_run' AND run->>'stage'='ready'
 AND run->>'v'=source_edition->>'version_id' AND final->>'v'=source_edition->>'version_id'
 AND plan->>'topic'=$6 AND plan->>'world_slug'=w.slug AND (plan->>'number')::int=$3
 AND plan->'spec' @> '{"publishToReader":true}' AND plan->'spec'->>'slug'=$4 AND plan->'spec'->>'sceneSlug'=$5
 AND chapter->>'topic'=$6 AND chapter->>'slug'=$4 AND chapter->>'scene_slug'=$5
 AND art->>'world_slug'=w.slug AND art->>'kind'='scene' AND art->>'slug'=$5
 AND media->>'art_job'=art->>'id' AND media->>'stage'='passed'
 AND media->>'slot'=final->'picks'->0->>'slot' AND media->>'round'=final->'picks'->0->>'round'
 AND media->>'variant'=final->'picks'->0->>'variant'
 AND NOT EXISTS (
  SELECT FROM jsonb_array_elements(chapter->'people') person WHERE NOT EXISTS (
   SELECT FROM jsonb_array_elements(portraits) portrait JOIN edition_reader_art a
    ON a.kind='character' AND a.world_slug=$2 AND a.item_slug=portrait->>'item_slug'
    AND a.image=portrait->>'image' AND a.source_sha256=portrait->>'source_sha256'
   WHERE portrait->>'item_slug'=person->>'slug'
  )
 )
)
