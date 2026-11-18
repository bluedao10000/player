package knowledge

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func uuid(s string) bool { var id pgtype.UUID; return len(s) == 36 && id.Scan(s) == nil }
func limit(c *gin.Context) int {
	n, e := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if e != nil || n < 1 || n > 100 {
		c.JSON(400, gin.H{"error": "limit must be 1..100"})
		return 0
	}
	return n
}
func queryJSON(c *gin.Context, db *pgxpool.Pool, sql string, args ...any) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	var result []byte
	err := db.QueryRow(ctx, sql, args...).Scan(&result)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(404, gin.H{"error": "work not found"})
		return
	}
	if err != nil {
		log.Printf("knowledge query: %v", err)
		c.JSON(503, gin.H{"error": "knowledge database unavailable"})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", result)
}
func Routes(r *gin.Engine, db *pgxpool.Pool) {
	adminRoutes(r, db)
	g := r.Group("/api/v1/knowledge")
	g.GET("/works", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		q := strings.TrimSpace(c.Query("q"))
		after := c.Query("after")
		if len(q) > 512 || (after != "" && !uuid(after)) {
			c.JSON(400, gin.H{"error": "invalid q or after"})
			return
		}
		queryJSON(c, db, `WITH page AS MATERIALIZED (
   SELECT w.id,w.canonical_title,w.original_title,w.type,w.episodes,w.status,w.season_year
   FROM works w WHERE active AND ($1='' OR strpos(lower(w.canonical_title),lower($1))>0 OR strpos(lower(w.original_title),lower($1))>0 OR EXISTS(SELECT FROM work_titles t WHERE t.work_id=w.id AND strpos(lower(t.title),lower($1))>0))
   AND (nullif($2,'') IS NULL OR w.id>nullif($2,'')::uuid) ORDER BY w.id LIMIT $3+1
  ), visible AS(SELECT * FROM page ORDER BY id LIMIT $3)
  SELECT jsonb_build_object('items',coalesce(jsonb_agg(to_jsonb(visible) ORDER BY id),'[]'),
   'next_cursor',CASE WHEN (SELECT count(*) FROM page)>$3 THEN max(id::text) END) FROM visible`, q, after, n)
	})
	g.GET("/works/:id", func(c *gin.Context) {
		id := c.Param("id")
		if !uuid(id) {
			c.JSON(400, gin.H{"error": "invalid work UUID"})
			return
		}
		queryJSON(c, db, `SELECT to_jsonb(w)||jsonb_build_object(
   'titles',coalesce((SELECT jsonb_agg(to_jsonb(t)-'work_id' ORDER BY lang,title) FROM work_titles t WHERE work_id=w.id),'[]'),
   'sources',coalesce((SELECT jsonb_agg(to_jsonb(s)-'work_id' ORDER BY provider,external_id,url) FROM work_sources s WHERE work_id=w.id),'[]'),
   'tags',coalesce((SELECT jsonb_agg(name ORDER BY name) FROM work_tags WHERE work_id=w.id),'[]'),
   'studios',coalesce((SELECT jsonb_agg(name ORDER BY name) FROM work_studios WHERE work_id=w.id),'[]'),
   'producers',coalesce((SELECT jsonb_agg(name ORDER BY name) FROM work_producers WHERE work_id=w.id),'[]'),
   'broadcasts',coalesce((SELECT jsonb_agg(detail ORDER BY detail) FROM work_broadcasts WHERE work_id=w.id),'[]'),
   'relations',coalesce((SELECT jsonb_agg(to_jsonb(x)-'work_id' ORDER BY url) FROM work_relations x WHERE work_id=w.id),'[]'),
   'provenance',coalesce((SELECT jsonb_agg(jsonb_build_object('source',s.name,'repository',s.repository_url,'version',v.version,'license',v.metadata->'license') ORDER BY s.name,r.record_key)
    FROM knowledge_source_records r JOIN knowledge_sources s ON s.id=r.source_id JOIN knowledge_source_versions v ON v.id=r.version_id WHERE r.active AND r.work_id=w.id),'[]')
   ) FROM works w WHERE id=$1 AND active`, id)
	})
	g.GET("/sources", func(c *gin.Context) {
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM
  (SELECT id,name,type,repository_url,enabled,sync_interval::text,last_checked_at,last_synced_at,current_version FROM knowledge_sources) x`)
	})
	g.GET("/quality-runs", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY started_at DESC),'[]') FROM
 (SELECT id,started_at,finished_at,status,report FROM knowledge_quality_runs ORDER BY started_at DESC LIMIT $1)x`, n)
	})
	g.GET("/sync-runs", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY started_at DESC),'[]') FROM
  (SELECT r.*,s.name AS source FROM knowledge_sync_runs r JOIN knowledge_sources s ON r.source_id=s.id WHERE $1='' OR s.name=$1 ORDER BY started_at DESC LIMIT $2)x`, c.Query("source"), n)
	})
	g.GET("/match-candidates", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x)),'[]') FROM
  (SELECT k.*,r.work_id,r.record_key FROM knowledge_match_candidates k JOIN knowledge_source_records r ON r.id=k.source_record_id ORDER BY k.source_record_id,k.candidate_work_id,k.reason LIMIT $1)x`, n)
	})
}
