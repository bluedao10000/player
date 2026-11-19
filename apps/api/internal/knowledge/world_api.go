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
	g.GET("/discovery", func(c *gin.Context) {
		queryJSON(c, db, `WITH chapters AS (
			SELECT v,position,card FROM published_chapters
			UNION ALL SELECT v,first_episode,chapter FROM published_extensions
		), scenes AS (
			SELECT s.v,c.position AS chapter,s.position,s.card FROM published_scenes s
			JOIN published_chapters c ON (c.v,c.slug)=(s.v,s.chapter)
			UNION ALL SELECT v,first_episode,0,scene FROM published_extensions
		), people AS (
			SELECT v,0 AS source,position,slug,card FROM published_characters
			UNION ALL SELECT v,1,0,slug,card FROM extension_people
		)
		SELECT jsonb_build_object('items',coalesce(jsonb_agg(w.card || jsonb_build_object(
			'chapters',coalesce(ch.cards,'[]'), 'characters',coalesce(p.cards,'[]'), 'scene',s.card
		) ORDER BY h.position),'[]'))
		FROM home_world_slots h JOIN published_worlds w USING(work_id)
		LEFT JOIN (SELECT DISTINCT ON (v) v,card FROM scenes ORDER BY v,chapter,position,card->>'slug') s ON s.v=w.version_id
		LEFT JOIN (SELECT v,jsonb_agg(card ORDER BY position,card->>'slug') cards FROM chapters GROUP BY v) ch ON ch.v=w.version_id
		LEFT JOIN (SELECT v,jsonb_agg(card ORDER BY source,position,slug) cards FROM people GROUP BY v) p ON p.v=w.version_id`)
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
		queryJSON(c, db, `SELECT editor_public_profile(p,$1,'world/setting') FROM editor_raw_overview($1) p WHERE p IS NOT NULL`, c.Param("work"))
	})
	g.GET("/worlds/:work/characters/:person", func(c *gin.Context) {
		queryJSON(c, db, `SELECT editor_public_profile(p,$1,'person/'||$2) FROM editor_raw_person($1,$2) p WHERE p IS NOT NULL`, c.Param("work"), c.Param("person"))
	})
	g.GET("/worlds/:work/chapters/:chapter", func(c *gin.Context) {
		queryJSON(c, db, `SELECT editor_public_page(p,$1) FROM editor_raw_chapter($1,$2) p WHERE p IS NOT NULL`, c.Param("work"), c.Param("chapter"))
	})
	g.GET("/worlds/:work/scenes/:scene", func(c *gin.Context) {
		queryJSON(c, db, `SELECT editor_public_page(p,$1) FROM editor_raw_scene($1,$2) p WHERE p IS NOT NULL`, c.Param("work"), c.Param("scene"))
	})
}
