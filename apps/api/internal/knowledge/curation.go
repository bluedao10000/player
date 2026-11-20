package knowledge

import (
	"anime-agent/api/internal/billing"
	"anime-agent/api/internal/reader"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"os"
	"strings"
)

type Decision struct {
	Action    string          `json:"action"`
	Reason    string          `json:"reason"`
	Work      string          `json:"work_id"`
	Field     string          `json:"field"`
	Value     json.RawMessage `json:"value"`
	Candidate int64           `json:"candidate_id"`
	Record    int64           `json:"record_id"`
}

var fields = map[string]string{"canonical_title": "title", "original_title": "original", "type": "type", "episodes": "episodes", "status": "status", "start_date": "start", "end_date": "end", "season": "season", "season_year": "year", "synopsis": "synopsis"}

func Decide(ctx context.Context, db *pgxpool.Pool, d Decision, operator, source string) error {
	if strings.TrimSpace(d.Reason) == "" || strings.TrimSpace(operator) == "" {
		return fmt.Errorf("operator and reason required")
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045102); CREATE TEMP TABLE affected(work_id uuid PRIMARY KEY) ON COMMIT DROP"); e != nil {
			return e
		}
		var old, next json.RawMessage
		var err error
		switch d.Action {
		case "override", "undo":
			key, ok := fields[d.Field]
			if !ok || !uuid(d.Work) {
				return fmt.Errorf("invalid field or work")
			}
			err = tx.QueryRow(ctx, `SELECT to_jsonb(w)->$2::text FROM works w WHERE id=$1 AND active`, d.Work, d.Field).Scan(&old)
			if err != nil {
				return err
			}
			if d.Action == "undo" {
				tag, e := tx.Exec(ctx, "DELETE FROM work_field_overrides WHERE work_id=$1 AND field=$2", d.Work, key)
				if e != nil {
					return e
				}
				if tag.RowsAffected() == 0 {
					return fmt.Errorf("no override")
				}
			} else {
				var v any
				if json.Unmarshal(d.Value, &v) != nil {
					return fmt.Errorf("invalid scalar")
				}
				switch v.(type) {
				case string:
					if key == "episodes" || key == "year" {
						return fmt.Errorf("integer required")
					}
				case float64:
					if key != "episodes" && key != "year" {
						return fmt.Errorf("text required")
					}
				default:
					return fmt.Errorf("scalar required")
				}
				if key == "title" && strings.TrimSpace(v.(string)) == "" {
					return fmt.Errorf("empty title")
				}
				_, err = tx.Exec(ctx, `INSERT INTO work_field_overrides VALUES($1,$2,$3) ON CONFLICT(work_id,field) DO UPDATE SET value=excluded.value`, d.Work, key, d.Value)
			}
			if err == nil {
				_, err = tx.Exec(ctx, "INSERT INTO affected VALUES($1)", d.Work)
			}
		case "approve", "reject", "merge", "split", "rematch":
			var from, to string
			if d.Action == "split" || d.Action == "rematch" {
				err = tx.QueryRow(ctx, "SELECT work_id::text FROM knowledge_source_records WHERE id=$1 AND active", d.Record).Scan(&from)
				to = from
			} else {
				err = tx.QueryRow(ctx, `SELECT r.id,r.work_id::text,k.candidate_work_id::text FROM knowledge_match_candidates k JOIN knowledge_source_records r ON r.id=k.source_record_id JOIN works w ON w.id=k.candidate_work_id WHERE k.id=$1 AND r.active AND w.active`, d.Candidate).Scan(&d.Record, &from, &to)
			}
			if err != nil {
				return err
			}
			restore := d.Work
			d.Work = from
			d.Field = "identity"
			if d.Action == "split" && restore != "" {
				err = tx.QueryRow(ctx, `SELECT work_id::text FROM knowledge_curation_actions WHERE action IN ('approve','merge') AND work_id=$1 AND after_value->>'work_id'=$2 AND before_value->'records' @> jsonb_build_array($3::bigint) LIMIT 1`, restore, from, d.Record).Scan(&to)
				if err != nil {
					return err
				}
			} else if d.Action == "split" {
				err = tx.QueryRow(ctx, "INSERT INTO works(canonical_title) SELECT canonical_title FROM works WHERE id=$1 RETURNING id::text", from).Scan(&to)
				if err != nil {
					return err
				}
			}
			_, err = tx.Exec(ctx, "INSERT INTO affected SELECT DISTINCT unnest($1::uuid[])", []string{from, to})
			if err != nil {
				return err
			}
			var ids []int64
			err = tx.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM knowledge_source_records WHERE (id=$1 OR ($2='merge' AND work_id=$3))", d.Record, d.Action, from).Scan(&ids)
			if err != nil {
				return err
			}
			old, _ = json.Marshal(gin.H{"work_id": from, "records": ids, "candidate_id": d.Candidate})
			switch d.Action {
			case "reject":
				_, err = tx.Exec(ctx, "INSERT INTO knowledge_match_rejections VALUES($1,$2) ON CONFLICT DO NOTHING", d.Record, to)
				if err == nil {
					_, err = tx.Exec(ctx, "DELETE FROM knowledge_match_candidates WHERE source_record_id=$1 AND candidate_work_id=$2", d.Record, to)
				}
			case "rematch":
				_, err = tx.Exec(ctx, "DELETE FROM knowledge_identity_pins WHERE record_id=$1", d.Record)
				if err == nil {
					_, err = tx.Exec(ctx, "DELETE FROM knowledge_match_rejections WHERE record_id=$1", d.Record)
				}
			default:
				_, err = tx.Exec(ctx, `INSERT INTO knowledge_identity_pins SELECT unnest($1::bigint[]),$2::uuid ON CONFLICT(record_id) DO UPDATE SET work_id=excluded.work_id`, ids, to)
				if err == nil {
					_, err = tx.Exec(ctx, "UPDATE knowledge_source_records SET work_id=$2 WHERE id=ANY($1)", ids, to)
				}
			}
			next, _ = json.Marshal(gin.H{"work_id": to, "records": ids, "decision": d.Action})
		default:
			return fmt.Errorf("unknown action")
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, projectionSQL); err != nil {
			return err
		}
		if d.Action == "override" || d.Action == "undo" {
			err = tx.QueryRow(ctx, "SELECT to_jsonb(w)->$2::text FROM works w WHERE id=$1", d.Work, d.Field).Scan(&next)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO knowledge_curation_actions(work_id,operator,action,field,before_value,after_value,reason,source) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, d.Work, operator, d.Action, d.Field, old, next, d.Reason, source)
		return err
	})
}
func adminRoutes(r *gin.Engine, db *pgxpool.Pool) {
	g := r.Group("/api/v1/admin/knowledge", func(c *gin.Context) {
		token, operator := os.Getenv("KNOWLEDGE_ADMIN_TOKEN"), os.Getenv("KNOWLEDGE_ADMIN_OPERATOR")
		if operator == "" || len(token) < 32 || subtle.ConstantTimeCompare([]byte(c.GetHeader("Authorization")), []byte("Bearer "+token)) != 1 {
			c.AbortWithStatus(401)
			return
		}
		c.Set("operator", operator)
		c.Header("Cache-Control", "no-store")
		c.Next()
	})
	editorRoutes(r, g, db)
	matchAIRoutes(g, db)
	usageRoutes(g, db)
	billing.AdminRoutes(g, db)
	reader.AdminRoutes(g, db)
	g.POST("/quality", func(c *gin.Context) {
		work := c.Query("work_id")
		if work != "" && !uuid(work) {
			c.Status(400)
			return
		}
		report, err := Quality(c.Request.Context(), db, c.DefaultQuery("source", "all"), work)
		if err != nil {
			log.Print(err)
			c.JSON(422, gin.H{"error": "quality scan failed or empty scope"})
			return
		}
		c.Data(200, "application/json", report)
	})
	g.GET("/versions", func(c *gin.Context) {
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id DESC),'[]') FROM (SELECT v.id,s.name,v.version,v.checksum,v.snapshot_key IS NOT NULL AS archived FROM knowledge_source_versions v JOIN knowledge_sources s ON s.id=v.source_id WHERE s.name=$1)x`, c.Query("source"))
	})
	g.GET("/matches", func(c *gin.Context) {
		n := limit(c)
		if n > 0 {
			queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x)),'[]') FROM (SELECT * FROM knowledge_match_candidates ORDER BY id LIMIT $1)x`, n)
		}
	})
	g.GET("/matches/:id", func(c *gin.Context) {
		queryJSON(c, db, `SELECT to_jsonb(k)||jsonb_build_object('incoming',r.raw,'candidate_records',(SELECT jsonb_agg(jsonb_build_object('id',x.id,'raw',x.raw,'diff',(SELECT coalesce(jsonb_object_agg(key,jsonb_build_array(r.raw->key,x.raw->key)),'{}') FROM jsonb_object_keys(r.raw||x.raw) key WHERE r.raw->key IS DISTINCT FROM x.raw->key))) FROM knowledge_source_records x WHERE x.active AND x.work_id=k.candidate_work_id)) FROM knowledge_match_candidates k JOIN knowledge_source_records r ON r.id=k.source_record_id WHERE k.id=$1`, c.Param("id"))
	})
	g.GET("/works/:id/issues", func(c *gin.Context) {
		queryJSON(c, db, `SELECT jsonb_build_object('score',(SELECT to_jsonb(s) FROM knowledge_quality_scores s WHERE work_id=$1),'issues',coalesce((SELECT jsonb_agg(to_jsonb(i)) FROM knowledge_quality_issues i WHERE entity_id=$1),'[]'))`, c.Param("id"))
	})
	g.GET("/works/:id/history", func(c *gin.Context) {
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY id DESC),'[]') FROM knowledge_curation_actions a WHERE work_id=$1 OR after_value->>'work_id'=$1::uuid::text`, c.Param("id"))
	})
	g.POST("/actions", func(c *gin.Context) {
		var d Decision
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
		if c.ShouldBindJSON(&d) != nil {
			c.Status(400)
			return
		}
		if e := Decide(c.Request.Context(), db, d, c.GetString("operator"), "admin-api"); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				c.Status(404)
			} else {
				log.Print(e)
				c.JSON(409, gin.H{"error": "invalid or conflicting decision"})
			}
			return
		}
		c.Status(204)
	})
}
