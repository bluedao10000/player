package knowledge

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	catalog "anime-agent/api/internal/model/knowledge"
	"anime-agent/api/internal/repository/workbench"
	queries "anime-agent/database/queries/catalog"
)

var assetName = regexp.MustCompile(`^((mio|bebop|bebop-portrait|bebop-e3|cfg-[1-9][0-9]*)-[1-9]-r[12](\.png|-crop[01]\.jpg)|bebop-reference-v1\.png)$`)

func mediaRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	mediaDir := os.Getenv("KNOWLEDGE_MEDIA_DIR")
	if mediaDir == "" {
		mediaDir = "/media"
	}
	g.GET("/media-candidates", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.MediaCandidates(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/reader-portraits/:file", func(c *gin.Context) {
		name := c.Param("file")
		if !catalog.PortraitFilename.MatchString(name) {
			c.Status(404)
			return
		}
		var published bool
		err := db.QueryRow(c.Request.Context(), queries.IsPublishedPortrait, "/art/portraits/"+name).Scan(&published)
		if err != nil {
			c.Status(503)
			return
		}
		if !published {
			c.Status(404)
			return
		}
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.File(filepath.Join(mediaDir, name))
	})
	g.GET("/media-assets/:file", func(c *gin.Context) {
		name := c.Param("file")
		if !assetName.MatchString(name) {
			c.Status(404)
			return
		}
		path := filepath.Join(mediaDir, name)
		if _, err := os.Stat(path); err != nil {
			c.Status(404)
			return
		}
		c.Header("Cache-Control", "private, max-age=86400, immutable")
		c.Header("X-Robots-Tag", "noindex")
		c.File(path)
	})
}
