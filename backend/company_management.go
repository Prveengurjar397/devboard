package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func platformAdminOnly(c *gin.Context) bool {
	if !isPlatformAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin access required"})
		return false
	}
	return true
}

// GET /platform/organizations
func platformListOrganizations(c *gin.Context) {
	if !platformAdminOnly(c) {
		return
	}

	rows, err := db.QueryContext(c.Request.Context(), `
		SELECT
			o.id,
			o.name,
			o.slug,
			o.is_active,
			o.created_at,
			o.updated_at,
			COUNT(DISTINCT om.user_id) AS member_count,
			COUNT(DISTINCT p.id) AS project_count,
			COUNT(DISTINCT i.id) FILTER (WHERE i.status <> 'resolved') AS open_incident_count
		FROM organizations o
		LEFT JOIN organization_members om ON om.organization_id = o.id
		LEFT JOIN projects p ON p.organization_id = o.id
		LEFT JOIN incidents i ON i.organization_id = o.id
		WHERE o.deleted_at IS NULL
		GROUP BY o.id
		ORDER BY o.created_at DESC
	`)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	type organizationSummary struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Slug              string `json:"slug"`
		IsActive          bool   `json:"is_active"`
		CreatedAt         string `json:"created_at"`
		UpdatedAt         string `json:"updated_at"`
		MemberCount       int    `json:"member_count"`
		ProjectCount      int    `json:"project_count"`
		OpenIncidentCount int    `json:"open_incident_count"`
	}

	result := make([]organizationSummary, 0)

	for rows.Next() {
		var item organizationSummary
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Slug,
			&item.IsActive,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.MemberCount,
			&item.ProjectCount,
			&item.OpenIncidentCount,
		); err != nil {
			fail(c, err)
			return
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"organizations": result})
}

// PATCH /platform/organizations/:id/status
func platformSetOrganizationStatus(c *gin.Context) {
	if !platformAdminOnly(c) {
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active is required"})
		return
	}

	var updatedID int
	err = db.QueryRowContext(c.Request.Context(), `
		UPDATE organizations
		SET
			is_active = $1,
			suspended_at = CASE WHEN $1 THEN NULL ELSE NOW() END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING id
	`, body.IsActive, id).Scan(&updatedID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}

	status := "suspended"
	if body.IsActive {
		status = "active"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "organization status updated",
		"id":      updatedID,
		"status":  status,
	})
}

// DELETE /platform/organizations/:id
// Soft delete: organization data remains in the database for recovery/audit.
func platformDeleteOrganization(c *gin.Context) {
	if !platformAdminOnly(c) {
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	result, err := db.ExecContext(c.Request.Context(), `
		UPDATE organizations
		SET
			is_active = FALSE,
			deleted_at = NOW(),
			suspended_at = COALESCE(suspended_at, NOW()),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
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
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "organization deleted"})
}

// GET /organizations/:id/status
func getOrganizationStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	if !isPlatformAdmin(c) {
		var allowed bool
		err = db.QueryRowContext(c.Request.Context(), `
			SELECT EXISTS (
				SELECT 1
				FROM organization_members
				WHERE organization_id = $1 AND user_id = $2
			)
		`, id, currentUserID(c)).Scan(&allowed)

		if err != nil {
			fail(c, err)
			return
		}
		if !allowed {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return
		}
	}

	var result struct {
		ID          int          `json:"id"`
		Name        string       `json:"name"`
		IsActive    bool         `json:"is_active"`
		SuspendedAt sql.NullTime `json:"-"`
		DeletedAt   sql.NullTime `json:"-"`
	}

	err = db.QueryRowContext(c.Request.Context(), `
		SELECT id, name, is_active, suspended_at, deleted_at
		FROM organizations
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&result.ID,
		&result.Name,
		&result.IsActive,
		&result.SuspendedAt,
		&result.DeletedAt,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}

	status := "active"
	if !result.IsActive {
		status = "suspended"
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        result.ID,
		"name":      result.Name,
		"is_active": result.IsActive,
		"status":    status,
		"suspended": result.SuspendedAt.Valid,
	})
}

// resolveChatOrganization validates the organization selected for chatbot access.
func resolveChatOrganization(c *gin.Context, requestedID int) (int, bool) {
	userID := currentUserID(c)

	if isPlatformAdmin(c) {
		if requestedID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "organization_id is required for platform admin",
			})
			return 0, false
		}

		var active bool
		err := db.QueryRowContext(c.Request.Context(), `
			SELECT is_active
			FROM organizations
			WHERE id = $1 AND deleted_at IS NULL
		`, requestedID).Scan(&active)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return 0, false
		}
		if err != nil {
			fail(c, err)
			return 0, false
		}
		if !active {
			c.JSON(http.StatusForbidden, gin.H{"error": "organization is suspended"})
			return 0, false
		}
		return requestedID, true
	}

	if requestedID > 0 {
		var active bool
		err := db.QueryRowContext(c.Request.Context(), `
			SELECT o.is_active
			FROM organizations o
			JOIN organization_members om ON om.organization_id = o.id
			WHERE o.id = $1
			  AND om.user_id = $2
			  AND o.deleted_at IS NULL
		`, requestedID, userID).Scan(&active)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return 0, false
		}
		if err != nil {
			fail(c, err)
			return 0, false
		}
		if !active {
			c.JSON(http.StatusForbidden, gin.H{"error": "organization is suspended"})
			return 0, false
		}
		return requestedID, true
	}

	rows, err := db.QueryContext(c.Request.Context(), `
		SELECT o.id, o.is_active
		FROM organizations o
		JOIN organization_members om ON om.organization_id = o.id
		WHERE om.user_id = $1 AND o.deleted_at IS NULL
		ORDER BY o.id
	`, userID)
	if err != nil {
		fail(c, err)
		return 0, false
	}
	defer rows.Close()

	var organizationID int
	count := 0
	active := false

	for rows.Next() {
		var id int
		var isActive bool
		if err := rows.Scan(&id, &isActive); err != nil {
			fail(c, err)
			return 0, false
		}
		count++
		organizationID = id
		active = isActive
	}
	if err := rows.Err(); err != nil {
		fail(c, err)
		return 0, false
	}

	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "no organization membership found"})
		return 0, false
	}
	if count > 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "organization_id is required when you belong to multiple organizations",
		})
		return 0, false
	}
	if !active {
		c.JSON(http.StatusForbidden, gin.H{"error": "organization is suspended"})
		return 0, false
	}

	return organizationID, true
}

var _ = strings.TrimSpace
