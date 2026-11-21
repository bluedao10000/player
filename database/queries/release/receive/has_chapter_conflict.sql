SELECT EXISTS(SELECT FROM published_chapters c JOIN published_worlds w ON c.v=w.version_id WHERE w.slug=$1 AND (c.slug=$2 OR c.episode=$4))
    OR EXISTS(SELECT FROM published_scenes s JOIN published_worlds w ON s.v=w.version_id WHERE w.slug=$1 AND s.slug=$3)
    OR EXISTS(SELECT FROM visible_reader_scopes WHERE world_slug=$1 AND $4 BETWEEN first_episode AND last_episode)
    OR EXISTS(SELECT FROM episode_extensions e JOIN research_topics t ON t.id=e.topic WHERE t.world_slug=$1 AND (e.slug=$2 OR e.scene_slug=$3))
