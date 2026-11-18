package knowledge

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"regexp"
	"strings"
)

var sceneFile = regexp.MustCompile(`^[a-f0-9]{64}\.jpg$`)

func worldRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.GET("/scope-acceptances", func(c *gin.Context) {
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(jsonb_build_object('id',a.id,'world',a.edition_slug,'edition_title',w.name||' · '||w.edition,'first',a.scope_first,'last',a.scope_last,
			'version',a.v,'rule',a.rule,'checked_at',a.checked_at,'published',p.id IS NOT NULL,'passed',a.passed,'checks',a.checks,'picks',a.picks,
            'previous_gaps',(SELECT jsonb_build_object('id',b.id,'items',(SELECT jsonb_agg((c->>'name')||' · '||(i->>'label')||'：'||(i->>'gap') ORDER BY dim,seq)
              FROM jsonb_array_elements(b.checks) WITH ORDINALITY dims(c,dim) CROSS JOIN jsonb_array_elements(c->'items') WITH ORDINALITY items(i,seq) WHERE i ? 'gap'))
              FROM scope_acceptances b WHERE b.topic=a.topic AND b.v=a.v AND b.rule=a.rule AND b.id<a.id AND NOT b.passed ORDER BY b.id DESC LIMIT 1)) ORDER BY h.topic),'[]')
			FROM scope_heads h JOIN scope_acceptances a ON h.latest=a.id JOIN research_topics t ON t.id=h.topic JOIN work_worlds w ON w.slug=t.world_slug LEFT JOIN visible_reader_scopes p ON p.id=a.id`)
	})
	g.GET("/episode-scenes/:file", func(c *gin.Context) {
		name := c.Param("file")
		if !sceneFile.MatchString(name) {
			c.Status(404)
			return
		}
		var published bool
		err := db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT FROM visible_reader_scopes s, jsonb_array_elements(s.picks) p WHERE p->>'image'=$1)`, "/art/episode-scenes/"+name).Scan(&published)
		if err != nil {
			c.Status(503)
			return
		}
		if !published {
			c.Status(404)
			return
		}
		path, _ := archivePath("sha256:" + strings.TrimSuffix(name, ".jpg"))
		c.Header("Cache-Control", "no-cache")
		c.File(path)
	})
	g.GET("/worlds", func(c *gin.Context) {
		if c.DefaultQuery("placement", "home") != "home" {
			c.JSON(400, gin.H{"error": "unknown placement"})
			return
		}
		queryJSON(c, db, `SELECT jsonb_build_object('items',coalesce(jsonb_agg(w.card || jsonb_build_object('scene_previews',coalesce((SELECT jsonb_agg(x.card ORDER BY x.slug) FROM (SELECT s.card,s.slug FROM published_scenes s WHERE s.v=w.version_id ORDER BY s.slug LIMIT 4)x),'[]')) ORDER BY h.position),'[]'))
    FROM home_world_slots h JOIN published_worlds w USING(work_id)`)
	})
	g.GET("/worlds/:work", func(c *gin.Context) {
		queryJSON(c, db, `SELECT jsonb_build_object('work',w.card,
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
    FROM published_worlds w WHERE w.slug=$1`, c.Param("work"))
	})
	g.GET("/worlds/:work/characters/:person", func(c *gin.Context) {
		queryJSON(c, db, `SELECT jsonb_build_object('work',w.card,'character',p.card,'intro',(SELECT i.people->>p.slug FROM edition_reader_intros i WHERE i.world_slug=w.slug),'scene_previews',coalesce((SELECT jsonb_agg(s.card ORDER BY s.slug) FROM scene_appearances a JOIN published_scenes s ON (a.v,a.scene)=(s.v,s.slug) WHERE (a.v,a.person)=(p.v,p.slug)),'[]') || coalesce((SELECT jsonb_agg(e.scene) FROM published_extensions e WHERE e.v=p.v AND p.slug='spike'),'[]'),'appearances',coalesce((
    SELECT jsonb_agg(c.card ORDER BY c.position) FROM chapter_appearances a JOIN published_chapters c ON (c.v,c.slug)=(a.v,a.chapter)
    WHERE (a.v,a.person)=(p.v,p.slug)),'[]') || coalesce((SELECT jsonb_agg(e.chapter ORDER BY e.first_episode) FROM published_extensions e WHERE e.v=p.v AND e.people @> jsonb_build_array(jsonb_build_object('slug',p.slug))),'[]')) FROM published_worlds w JOIN published_characters p ON p.v=w.version_id WHERE w.slug=$1 AND p.slug=$2
    UNION ALL SELECT jsonb_build_object('work',w.card,'character',p.card,'intro',p.intro,
      'scene_previews',coalesce((SELECT jsonb_agg(x.scene ORDER BY x.first_episode) FROM published_extensions x JOIN episode_plans plan USING(topic) WHERE x.v=p.v AND x.people @> jsonb_build_array(jsonb_build_object('slug',p.slug)) AND strpos(plan.spec->'art'->>'brief',p.card->>'name')>0),'[]'),
      'appearances',(SELECT jsonb_agg(x.chapter ORDER BY x.first_episode) FROM published_extensions x WHERE x.v=p.v AND x.people @> jsonb_build_array(jsonb_build_object('slug',p.slug))))
    FROM extension_people p JOIN published_extensions e ON e.id=p.id JOIN published_worlds w ON w.version_id=p.v WHERE w.slug=$1 AND p.slug=$2`, c.Param("work"), c.Param("person"))
	})
	g.GET("/worlds/:work/chapters/:chapter", func(c *gin.Context) {
		queryJSON(c, db, `SELECT jsonb_build_object('work',w.card,'chapter',c.card,'accepted_art',(SELECT jsonb_agg(p-ARRAY['sha','slot','round','variant','phone','phone_sha'] ORDER BY ord) FROM visible_reader_scopes s CROSS JOIN jsonb_array_elements(s.picks) WITH ORDINALITY e(p,ord) WHERE s.world_slug=w.slug AND s.v=c.v AND c.episode BETWEEN s.first_episode AND s.last_episode AND p->>'chapter'=c.slug),
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
    FROM published_extensions e JOIN published_worlds w ON w.version_id=e.v JOIN research_runs r ON r.id=e.research_run WHERE w.slug=$1 AND e.slug=$2`, c.Param("work"), c.Param("chapter"))
	})
	g.GET("/worlds/:work/scenes/:scene", func(c *gin.Context) {
		queryJSON(c, db, `SELECT jsonb_build_object('work',w.card,'scene',s.card||jsonb_build_object('text',s.text),'chapter',c.card,'setting',s.place,
    'scene_cast',coalesce((SELECT jsonb_agg(p.card ORDER BY p.position) FROM scene_appearances a JOIN published_characters p ON (p.v,p.slug)=(a.v,a.person)
     WHERE (a.v,a.scene)=(s.v,s.slug)),'[]'),
    'return_to',jsonb_build_object('block_key',s.block,'href',(c.card->>'href')||'#passage-'||s.slug),
    'attribution',(SELECT jsonb_agg(jsonb_build_object('uri',e.uri,'locator',e.locator) ORDER BY e.key) FROM world_evidence e WHERE e.v=s.v AND
      (e.key=s.evidence OR EXISTS(SELECT FROM scene_appearances a WHERE (a.v,a.scene,a.evidence)=(e.v,s.slug,e.key)))))
    FROM published_worlds w JOIN published_scenes s ON s.v=w.version_id JOIN published_chapters c ON (c.v,c.slug)=(s.v,s.chapter) WHERE w.slug=$1 AND s.slug=$2
    UNION ALL SELECT jsonb_build_object('work',w.card,'chapter',e.chapter,'setting',e.place,'scene',e.scene||jsonb_build_object('text',e.reader_copy->e.slug->coalesce((plan.spec->>'sceneBeat')::int-1,1)->>'text'),
      'scene_cast',coalesce((SELECT jsonb_agg(card ORDER BY ord) FROM extension_cast c WHERE c.topic=e.topic AND ((plan.id IS NULL AND c.slug='spike') OR (plan.id IS NOT NULL AND strpos(plan.spec->'art'->>'brief',c.card->>'name')>0))),'[]'::jsonb),
      'return_to',jsonb_build_object('block_key',e.picks->0->>'block','href',(e.chapter->>'href')||'#passage-'||e.scene_slug),'attribution','[]'::jsonb)
    FROM published_extensions e JOIN published_worlds w ON w.version_id=e.v LEFT JOIN episode_plans plan USING(topic) WHERE w.slug=$1 AND e.scene_slug=$2`, c.Param("work"), c.Param("scene"))
	})
}
