package knowledge

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	reads "anime-agent/api/internal/repository/catalog"
)

func worldRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.GET("/scope-acceptances", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.ScopeAcceptances(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/discovery", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.Discovery(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/worlds", func(c *gin.Context) {
		if c.DefaultQuery("placement", "home") != "home" {
			c.JSON(400, gin.H{"error": "unknown placement"})
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.HomeWorlds(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/worlds/:work", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.World(ctx, db, c.Param("work"))
		writeJSON(c, body, err)
	})
	g.GET("/worlds/:work/characters/:person", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.Character(ctx, db, c.Param("work"), c.Param("person"))
		writeJSON(c, body, err)
	})
	g.GET("/worlds/:work/chapters/:chapter", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.Chapter(ctx, db, c.Param("work"), c.Param("chapter"))
		writeJSON(c, body, err)
	})
	g.GET("/worlds/:work/scenes/:scene", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.Scene(ctx, db, c.Param("work"), c.Param("scene"))
		writeJSON(c, body, err)
	})
}
