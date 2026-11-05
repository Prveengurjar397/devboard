package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func MonitoringAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.Fields(c.GetHeader("Authorization"))

		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"error": "monitoring token required"})
			return
		}

		sum := sha256.Sum256([]byte(header[1]))
		tokenHash := hex.EncodeToString(sum[:])

		var sourceID int64
		var orgID int
		err := db.QueryRowContext(c.Request.Context(), `
			SELECT ms.id, ms.organization_id
			FROM monitoring_sources ms
			JOIN organizations o ON o.id = ms.organization_id
			WHERE ms.token_hash = $1
			  AND ms.is_enabled = TRUE
			  AND ms.token_revoked_at IS NULL
			  AND o.is_active = TRUE
			  AND o.deleted_at IS NULL
		`, tokenHash).Scan(&sourceID, &orgID)

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"error": "invalid, revoked, or disabled monitoring token"})
			return
		}

		_, err = db.ExecContext(c.Request.Context(),
			`UPDATE monitoring_sources SET last_seen_at=NOW() WHERE id=$1`,
			sourceID,
		)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{"error": "could not update monitoring source"})
			return
		}

		c.Set("monitoring_source_id", sourceID)
		c.Set("monitoring_organization_id", orgID)
		c.Next()
	}
}
