package knowledge

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed worlddata/*.json
var worldFiles embed.FS

type worldProof struct{ URI, Locator, Digest string }
type worldPerson struct {
	Slug, Name, Role, Portrait, Href, Alias, OriginalName string
	ProviderID                                            int `json:"provider_id"`
}
type worldScene struct {
	Slug, Name, Place, Block, Text string
	Cast                           []string
}
type worldBlock struct{ Key, Text string }
type worldChapter struct {
	Slug, Name, Label, Intro, Href, Image, OriginalName string
	Episode, ProviderID                                 int
	Cast                                                []string
	Blocks                                              []worldBlock
	Scenes                                              []worldScene
}
type worldLine struct {
	Slug, Title, Intro string
	Chapters           []string
}
type worldManifest struct {
	Schema, Position                  int
	Slug, Name, Edition, Intro, Image string
	CastTitle                         string `json:"cast_title"`
	People                            []worldPerson
	Lines                             []worldLine
	Chapters                          []worldChapter
	Proofs                            map[string]worldProof
}

var worldSlug = regexp.MustCompile("^[a-z0-9]+(-[a-z0-9]+)*$")

func digest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

func loadWorld(ctx context.Context, conn *pgx.Conn, s source, st *stats) (worldManifest, error) {
	var w worldManifest
	data, err := worldFiles.ReadFile("worlddata/" + s.Asset)
	if err != nil {
		return w, err
	}
	if err = json.Unmarshal(data, &w); err != nil {
		return w, err
	}
	w.Proofs = map[string]worldProof{"editorial": {URI: "local:worlddata/" + s.Asset, Locator: "manifest", Digest: digest(data)}}
	if s.Kind == "world-enrichment" {
		fetcher := worldFetcher{conn: conn, sourceID: s.ID, changed: make(map[string]cachedWorldDocument)}
		if err = enrichBebop(ctx, s, &w, st, &fetcher); err != nil {
			return w, err
		}
		for url, doc := range fetcher.changed {
			_, err = conn.Exec(ctx, `INSERT INTO world_fetch_cache VALUES($1,$2,$3,$4,$5) ON CONFLICT(source_id,url) DO UPDATE SET etag=excluded.etag,modified=excluded.modified,body=excluded.body`, s.ID, url, doc.etag, doc.modified, doc.body)
			if err != nil {
				return w, err
			}
		}
	}
	return w, err
}

func (w worldManifest) validate() error {
	if w.Schema != 1 || !worldSlug.MatchString(w.Slug) || w.Name == "" || w.Position < 1 || len(w.People) == 0 || len(w.Chapters) == 0 || len(w.Lines) == 0 {
		return errors.New("incomplete world manifest")
	}
	for _, p := range w.People {
		if !worldSlug.MatchString(p.Slug) || p.Name == "" || (p.Href != "" && p.Href != "/character/"+p.Slug) {
			return fmt.Errorf("invalid character %q", p.Slug)
		}
	}
	for _, c := range w.Chapters {
		if !worldSlug.MatchString(c.Slug) || c.Name == "" || len(c.Blocks) == 0 || len(c.Cast) == 0 || (c.Href != "" && c.Href != "/story/"+c.Slug) {
			return fmt.Errorf("invalid chapter %q", c.Slug)
		}
		for _, b := range c.Blocks {
			if !worldSlug.MatchString(b.Key) || strings.TrimSpace(b.Text) == "" {
				return fmt.Errorf("invalid block in %s", c.Slug)
			}
		}
		for _, sc := range c.Scenes {
			if !worldSlug.MatchString(sc.Slug) || sc.Name == "" || sc.Text == "" || sc.Place == "" {
				return fmt.Errorf("invalid scene in %s", c.Slug)
			}
		}
	}
	return nil
}

func importWorld(ctx context.Context, conn *pgx.Conn, s source, run string, st *stats, replay string) error {
	if s.Kind == "world-enrichment" && replay == "" {
		var deferred bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT FROM world_source_policy WHERE source_id=$1 AND retry_after>now())", s.ID).Scan(&deferred); err != nil {
			return err
		}
		if deferred {
			_, err := conn.Exec(ctx, `UPDATE knowledge_sync_runs SET status='success',finished_at=now(),skipped=true,skip_reason='rate_limited' WHERE id=$1`, run)
			return err
		}
	}
	var w worldManifest
	var err error
	if replay != "" {
		var raw []byte
		err = conn.QueryRow(ctx, `SELECT metadata->'manifest' FROM knowledge_source_versions WHERE source_id=$1 AND version=coalesce(nullif($2,'@current'),$3)`, s.ID, replay, s.Local).Scan(&raw)
		if err == nil {
			err = json.Unmarshal(raw, &w)
		}
	} else {
		w, err = loadWorld(ctx, conn, s, st)
	}
	if err != nil {
		return err
	}
	if err = w.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(w)
	if err != nil {
		return err
	}
	st.Remote = digest(data)
	st.Checksum = st.Remote
	st.URL = s.DataURL
	st.Read = 1
	st.Length = int64(len(data))
	if st.Remote == s.Local {
		_, err = conn.Exec(ctx, `UPDATE knowledge_sync_runs SET status='success',finished_at=now(),skipped=true,skip_reason='version_unchanged',remote_version=$2,checksum=$2,records_read=1,records_unchanged=1 WHERE id=$1`, run, st.Remote)
		return err
	}
	// One staging row represents the whole work, like one upstream work record.
	_, err = conn.Exec(ctx, `INSERT INTO staging_source_records(batch_id,record_key,raw,norm) VALUES($1,$2,$3::jsonb,$3::jsonb)`, run, w.Slug, data)
	if err != nil {
		return err
	}
	err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045102)"); e != nil {
			return e
		}
		var v int64
		e := tx.QueryRow(ctx, `INSERT INTO knowledge_source_versions(source_id,version,checksum,metadata,parser_type,asset_name,run_id)
    SELECT $1,$2,$2,jsonb_build_object('manifest',norm),$3,$4,$5 FROM staging_source_records WHERE batch_id=$5 AND record_key=$6
    ON CONFLICT(source_id,version) DO UPDATE SET checksum=excluded.checksum RETURNING id`, s.ID, st.Remote, s.Kind, s.Asset, run, w.Slug).Scan(&v)
		if e != nil {
			return e
		}
		work, e := resolveWorld(ctx, tx, w, s, v)
		if e != nil {
			return e
		}
		var inserted int
		e = tx.QueryRow(ctx, `INSERT INTO world_releases VALUES($1,$2) ON CONFLICT DO NOTHING RETURNING 1`, v, work).Scan(&inserted)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if inserted == 1 {
			if e = storeWorldGraph(ctx, tx, w, v, s.Kind == "world-enrichment"); e != nil {
				return e
			}
		}
		origin := "original"
		if s.Kind == "world-enrichment" {
			origin = "external"
		}
		_, e = tx.Exec(ctx, `INSERT INTO work_worlds(work_id,slug,version_id,name,edition,intro,cast_title,image,origin)
    VALUES($1,$2,$3,$4,$5,$6,$7,nullif($8,''),$9) ON CONFLICT(work_id) DO UPDATE SET version_id=excluded.version_id,
    name=excluded.name,edition=excluded.edition,intro=excluded.intro,cast_title=excluded.cast_title,image=excluded.image,published_at=now(),revision=work_worlds.revision+1`, work, w.Slug, v, w.Name, w.Edition, w.Intro, w.CastTitle, w.Image, origin)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO home_world_slots VALUES($1,$2) ON CONFLICT(work_id) DO UPDATE SET position=excluded.position`, w.Position, work)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE knowledge_sources SET current_version=$2,last_synced_at=now() WHERE id=$1`, s.ID, st.Remote)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `DELETE FROM staging_source_records WHERE batch_id=$1`, run)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE knowledge_sync_runs SET status='success',finished_at=now(),remote_version=$2,checksum=$2,download_url=$3,
    content_length=$4,http_status=nullif($5,0),records_read=1,records_created=(local_version IS NULL)::int,records_updated=(local_version IS NOT NULL)::int WHERE id=$1`, run, st.Remote, st.URL, st.Length, st.HTTP)
		return e
	})
	if err != nil {
		st.Failed = 1
	}
	return err
}

func resolveWorld(ctx context.Context, tx pgx.Tx, w worldManifest, s source, v int64) (string, error) {
	var id string
	if s.Kind == "world-enrichment" {
		err := tx.QueryRow(ctx, `SELECT DISTINCT w.id::text FROM works w JOIN work_sources s ON w.id=s.work_id WHERE w.active AND w.type='TV' AND w.season_year=1998
   AND s.provider='bangumi' AND s.external_id='253' GROUP BY w.id HAVING (SELECT count(DISTINCT x.work_id) FROM work_sources x WHERE x.provider='bangumi' AND x.external_id='253')=1`).Scan(&id)
		if err != nil {
			return "", fmt.Errorf("resolve unique 1998 TV subject 253: %w", err)
		}
		return id, nil
	}
	err := tx.QueryRow(ctx, `SELECT work_id::text FROM knowledge_source_records WHERE source_id=$1 AND record_key=$2`, s.ID, w.Slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO works(canonical_title,type,synopsis) VALUES($1,'SHORT_STORY_COLLECTION',$2) RETURNING id::text`, w.Name, w.Intro).Scan(&id)
	}
	if err != nil {
		return "", err
	}
	norm := normalized{Title: w.Name, Original: w.Name, Type: "SHORT_STORY_COLLECTION", Synopsis: w.Intro, Titles: []title{{Lang: "zh", Title: w.Name}}}
	_, err = tx.Exec(ctx, `INSERT INTO knowledge_source_records(source_id,record_key,work_id,version_id,raw,norm)
 SELECT $1,$2,$3,$4,metadata->'manifest',$5 FROM knowledge_source_versions WHERE id=$4 ON CONFLICT(source_id,record_key) DO UPDATE SET version_id=excluded.version_id,raw=excluded.raw,norm=excluded.norm,active=true`, s.ID, w.Slug, id, v, norm)
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `CREATE TEMP TABLE affected(work_id uuid PRIMARY KEY) ON COMMIT DROP`); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO affected VALUES($1)`, id); err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, projectionSQL)
	return id, err
}

func storeWorldGraph(ctx context.Context, tx pgx.Tx, w worldManifest, v int64, external bool) error {
	b := &pgx.Batch{}
	for k, p := range w.Proofs {
		b.Queue("INSERT INTO world_evidence VALUES($1,$2,$3,$4,$5)", v, k, p.URI, p.Locator, p.Digest)
	}
	for i, p := range w.People {
		proof := "editorial"
		if external {
			proof = "character/" + p.Slug
		}
		b.Queue(`INSERT INTO work_characters VALUES($1,$2,$3,$4,nullif($5,''),nullif($6,''),$7,nullif($8,0)::text,nullif($9,''),$10)`, v, p.Slug, p.Name, p.Role, p.Portrait, p.Href, i, p.ProviderID, p.OriginalName, proof)
	}
	for i, c := range w.Chapters {
		proof, identity := "editorial", "editorial"
		if external {
			proof = "episode/" + c.Slug
			identity = "catalog/" + c.Slug
		}
		b.Queue(`INSERT INTO work_chapters VALUES($1,$2,$3,$4,$5,nullif($6,''),nullif($7,''),$8,nullif($9,0),nullif($10,0)::text,nullif($11,''),$12)`, v, c.Slug, c.Name, c.Label, c.Intro, c.Href, c.Image, i, c.Episode, c.ProviderID, c.OriginalName, identity)
		for j, p := range c.Blocks {
			b.Queue("INSERT INTO chapter_blocks VALUES($1,$2,$3,$4,$5)", v, c.Slug, p.Key, p.Text, j)
		}
		for _, p := range c.Cast {
			appearance := proof
			if external && c.Episode == 2 && (p == "jet" || p == "hakim") {
				appearance = "chase-account"
			}
			b.Queue("INSERT INTO chapter_appearances VALUES($1,$2,$3,$4)", v, c.Slug, p, appearance)
		}
		for j, sc := range c.Scenes {
			b.Queue("INSERT INTO work_settings VALUES($1,$2) ON CONFLICT DO NOTHING", v, sc.Place)
			b.Queue("INSERT INTO work_scenes VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", v, sc.Slug, c.Slug, sc.Name, sc.Place, sc.Text, sc.Block, j, proof)
			for _, p := range sc.Cast {
				sceneProof := proof
				if external && c.Episode == 2 {
					sceneProof = "chase-account"
				}
				b.Queue("INSERT INTO scene_appearances VALUES($1,$2,$3,$4,$5)", v, c.Slug, sc.Slug, p, sceneProof)
			}
		}
	}
	for i, line := range w.Lines {
		b.Queue("INSERT INTO work_storylines VALUES($1,$2,$3,$4,$5)", v, line.Slug, line.Title, line.Intro, i)
		for j, c := range line.Chapters {
			b.Queue("INSERT INTO work_storyline_chapters VALUES($1,$2,$3,$4)", v, line.Slug, c, j)
		}
	}
	return tx.SendBatch(ctx, b).Close()
}
