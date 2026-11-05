package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func generateMonitoringToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])

	return token, hash, nil
}

func parseMonitoringSourceID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monitoring source id"})
		return 0, false
	}
	return id, true
}

func monitoringSourceOrganization(c *gin.Context, sourceID int64) (int, bool) {
	var orgID int

	err := db.QueryRowContext(c.Request.Context(),
		`SELECT organization_id
		 FROM monitoring_sources
		 WHERE id=$1`,
		sourceID,
	).Scan(&orgID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "monitoring source not found"})
		return 0, false
	}

	if !authorizeOrganizationManager(c, orgID) {
		return 0, false
	}

	return orgID, true
}

func activeMonitoringOrganization(c *gin.Context, orgID int) bool {
	var active bool

	err := db.QueryRowContext(c.Request.Context(),
		`SELECT is_active AND deleted_at IS NULL
		 FROM organizations
		 WHERE id=$1`,
		orgID,
	).Scan(&active)

	if err != nil || !active {
		c.JSON(http.StatusConflict, gin.H{"error": "organization is inactive or unavailable"})
		return false
	}

	return true
}

// POST /organizations/:id/monitoring-sources
func createMonitoringSource(c *gin.Context) {
	orgID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orgID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	if !authorizeOrganizationManager(c, orgID) {
		return
	}
	if !activeMonitoringOrganization(c, orgID) {
		return
	}

	var body struct {
		Name       string `json:"name"`
		SourceType string `json:"source_type"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	body.SourceType = strings.ToLower(strings.TrimSpace(body.SourceType))

	if body.SourceType == "" {
		body.SourceType = "docker"
	}

	if body.Name == "" || len(body.Name) > 150 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source name is required and must be at most 150 characters"})
		return
	}
	if body.SourceType != "docker" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "currently only docker sources are supported"})
		return
	}

	token, tokenHash, err := generateMonitoringToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate monitoring token"})
		return
	}

	var sourceID int64
	err = db.QueryRowContext(c.Request.Context(),
		`INSERT INTO monitoring_sources
		 (organization_id, name, source_type, token_hash, token_created_at)
		 VALUES ($1,$2,$3,$4,NOW())
		 RETURNING id`,
		orgID, body.Name, body.SourceType, tokenHash,
	).Scan(&sourceID)

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "could not create source; the name may already exist"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          sourceID,
		"name":        body.Name,
		"source_type": body.SourceType,
		"token":       token,
		"message":     "Save this token now. It will not be shown again.",
	})
}

// GET /organizations/:id/monitoring-sources
func listMonitoringSources(c *gin.Context) {
	orgID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orgID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	if !authorizeOrganizationManager(c, orgID) {
		return
	}

	rows, err := db.QueryContext(c.Request.Context(),
		`SELECT id, name, source_type, is_enabled, last_seen_at, created_at,
		        token_created_at, token_revoked_at
		 FROM monitoring_sources
		 WHERE organization_id=$1
		 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var name, sourceType string
		var enabled bool
		var lastSeen, tokenCreated, tokenRevoked interface{}
		var createdAt interface{}

		if err := rows.Scan(
			&id, &name, &sourceType, &enabled, &lastSeen,
			&createdAt, &tokenCreated, &tokenRevoked,
		); err != nil {
			fail(c, err)
			return
		}

		items = append(items, gin.H{
			"id":               id,
			"name":             name,
			"source_type":      sourceType,
			"is_enabled":       enabled,
			"last_seen_at":     lastSeen,
			"created_at":       createdAt,
			"token_created_at": tokenCreated,
			"token_revoked_at": tokenRevoked,
		})
	}

	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"sources": items})
}

// POST /monitoring-sources/:id/rotate-token
func rotateMonitoringSourceToken(c *gin.Context) {
	sourceID, ok := parseMonitoringSourceID(c)
	if !ok {
		return
	}

	orgID, ok := monitoringSourceOrganization(c, sourceID)
	if !ok {
		return
	}
	if !activeMonitoringOrganization(c, orgID) {
		return
	}

	token, tokenHash, err := generateMonitoringToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate monitoring token"})
		return
	}

	result, err := db.ExecContext(c.Request.Context(),
		`UPDATE monitoring_sources
		 SET token_hash=$1, token_created_at=NOW(), token_revoked_at=NULL
		 WHERE id=$2 AND organization_id=$3 AND is_enabled=TRUE`,
		tokenHash, sourceID, orgID,
	)
	if err != nil {
		fail(c, err)
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		fail(c, err)
		return
	}
	if affected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "source is disabled and cannot rotate its token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      sourceID,
		"token":   token,
		"message": "Previous token is invalid. Save this new token now.",
	})
}

// DELETE /monitoring-sources/:id
func disableMonitoringSource(c *gin.Context) {
	sourceID, ok := parseMonitoringSourceID(c)
	if !ok {
		return
	}

	orgID, ok := monitoringSourceOrganization(c, sourceID)
	if !ok {
		return
	}

	result, err := db.ExecContext(c.Request.Context(),
		`UPDATE monitoring_sources
		 SET is_enabled=FALSE,
		     token_hash=NULL,
		     token_revoked_at=NOW()
		 WHERE id=$1 AND organization_id=$2`,
		sourceID, orgID,
	)
	if err != nil {
		fail(c, err)
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		fail(c, err)
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "monitoring source not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "monitoring source disabled and token revoked"})
}
