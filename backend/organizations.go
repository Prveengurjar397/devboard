package main

import (
	"database/sql"
	"net/http"
	"time"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func isPlatformAdmin(c *gin.Context) bool {
	var isAdmin bool
	err := db.QueryRowContext(
		c.Request.Context(),
		`SELECT EXISTS(
			SELECT 1 FROM users
			WHERE id=$1 AND platform_role='platform_admin'
		)`,
		currentUserID(c),
	).Scan(&isAdmin)
	return err == nil && isAdmin
}

func listOrganizations(c *gin.Context) {
	userID := currentUserID(c)

	query := `
		SELECT o.id, o.name, o.slug,
		       COALESCE(m.role, 'platform_admin') AS role
		FROM organizations o
		LEFT JOIN organization_members m
		  ON m.organization_id=o.id AND m.user_id=$1
		WHERE m.user_id IS NOT NULL`

	args := []interface{}{userID}

	if isPlatformAdmin(c) {
		query = `
			SELECT o.id, o.name, o.slug, 'platform_admin'
			FROM organizations o`
		args = nil
	}

	query += ` ORDER BY o.id`

	rows, err := db.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	items := []gin.H{}
	for rows.Next() {
		var id int
		var name, slug, role string
		if err := rows.Scan(&id, &name, &slug, &role); err != nil {
			fail(c, err)
			return
		}
		items = append(items, gin.H{
			"id": id, "name": name, "slug": slug, "role": role,
		})
	}
	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"organizations": items})
}

func createOrganization(c *gin.Context) {
	if !isPlatformAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only platform admin can create organizations directly"})
		return
	}
	var body struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	body.Slug = strings.ToLower(strings.TrimSpace(body.Slug))
	if body.Name == "" || len(body.Name) > 150 ||
		body.Slug == "" || len(body.Slug) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid name and slug are required"})
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		fail(c, err)
		return
	}
	defer tx.Rollback()

	var orgID int
	err = tx.QueryRowContext(c.Request.Context(),
		`INSERT INTO organizations(name, slug)
		 VALUES($1,$2) RETURNING id`,
		body.Name, body.Slug,
	).Scan(&orgID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "organization slug already exists"})
		return
	}

	_, err = tx.ExecContext(c.Request.Context(),
		`INSERT INTO organization_members
		 (organization_id,user_id,role,can_access_incidents)
		 VALUES($1,$2,'company_admin',TRUE)`,
		orgID, currentUserID(c),
	)
	if err != nil {
		fail(c, err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id": orgID, "name": body.Name, "slug": body.Slug,
		"role": "company_admin",
	})
}

func addOrganizationMember(c *gin.Context) {
	orgID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orgID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	var body struct {
		Email              string `json:"email"`
		Role               string `json:"role"`
		CanAccessIncidents bool   `json:"can_access_incidents"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	platformAdmin := isPlatformAdmin(c)

	switch body.Role {
	case "developer", "tester", "devops":
	case "company_admin":
		if !platformAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "only platform admin can assign company_admin role",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}

	var allowed bool
	if platformAdmin {
		allowed = true
	} else {
		err = db.QueryRowContext(c.Request.Context(),
			`SELECT EXISTS(
				SELECT 1 FROM organization_members
				WHERE organization_id=$1 AND user_id=$2
				  AND role='company_admin'
			)`,
			orgID, currentUserID(c),
		).Scan(&allowed)
		if err != nil {
			fail(c, err)
			return
		}
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "company admin access required"})
		return
	}

	var memberID int
	err = db.QueryRowContext(c.Request.Context(),
		`SELECT id FROM users
		 WHERE lower(email)=$1 AND is_legacy=FALSE`,
		body.Email,
	).Scan(&memberID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "registered user not found"})
		return
	}

	var targetIsPlatformAdmin bool
	err = db.QueryRowContext(c.Request.Context(),
		`SELECT platform_role='platform_admin' FROM users WHERE id=$1`,
		memberID,
	).Scan(&targetIsPlatformAdmin)
	if err != nil {
		fail(c, err)
		return
	}
	if targetIsPlatformAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin cannot be added as a company member"})
		return
	}

	_, err = db.ExecContext(c.Request.Context(),
		`INSERT INTO organization_members
		 (organization_id,user_id,role,can_access_incidents)
		 VALUES($1,$2,$3,$4)
		 ON CONFLICT(organization_id,user_id)
		 DO UPDATE SET role=EXCLUDED.role,
		               can_access_incidents=EXCLUDED.can_access_incidents`,
		orgID, memberID, body.Role, body.CanAccessIncidents,
	)
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "member added",
		"user_id": memberID,
		"role": body.Role,
		"can_access_incidents": body.CanAccessIncidents,
	})
}


// canAccessIncidentOrg checks whether a user can access incidents in an organization.
// Platform admins bypass organization membership checks.
func canAccessIncidentOrg(userID, orgID int) (bool, error) {
    var allowed bool
    err := db.QueryRow(
        `SELECT
            EXISTS(
                SELECT 1 FROM users
                WHERE id = $1 AND platform_role = 'platform_admin'
            )
            OR EXISTS(
                SELECT 1 FROM organization_members
                WHERE user_id = $1
                  AND organization_id = $2
                  AND (role = 'company_admin' OR can_access_incidents = TRUE)
            )`,
        userID, orgID,
    ).Scan(&allowed)
    return allowed, err
}


// authorizeOrganizationManager allows platform admins globally and company admins
// only inside their own organization.
func authorizeOrganizationManager(c *gin.Context, orgID int) bool {
	if isPlatformAdmin(c) {
		return true
	}

	var allowed bool
	err := db.QueryRowContext(c.Request.Context(),
		`SELECT EXISTS(
			SELECT 1 FROM organization_members
			WHERE organization_id=$1 AND user_id=$2 AND role='company_admin'
		)`, orgID, currentUserID(c),
	).Scan(&allowed)
	if err != nil {
		fail(c, err)
		return false
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "company admin access required"})
		return false
	}
	return true
}

// GET /organizations/:id/members
func listOrganizationMembers(c *gin.Context) {
	orgID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orgID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}
	if !authorizeOrganizationManager(c, orgID) {
		return
	}

	rows, err := db.QueryContext(c.Request.Context(),
		`SELECT u.id, u.name, u.email, m.role, m.can_access_incidents, m.created_at
		 FROM organization_members m
		 JOIN users u ON u.id=m.user_id
		 WHERE m.organization_id=$1
		 ORDER BY m.created_at, u.id`, orgID)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	members := []gin.H{}
	for rows.Next() {
		var id int
		var name, email, role string
		var canAccess bool
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &email, &role, &canAccess, &createdAt); err != nil {
			fail(c, err)
			return
		}
		members = append(members, gin.H{
			"user_id": id,
			"name": name,
			"email": email,
			"role": role,
			"can_access_incidents": canAccess,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

// PATCH /organizations/:id/members/:userId
func updateOrganizationMember(c *gin.Context) {
	orgID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orgID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}
	targetID, err := strconv.Atoi(c.Param("userId"))
	if err != nil || targetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if !authorizeOrganizationManager(c, orgID) {
		return
	}

	var body struct {
		Role *string `json:"role"`
		CanAccessIncidents *bool `json:"can_access_incidents"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if body.Role == nil && body.CanAccessIncidents == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide role or can_access_incidents"})
		return
	}

	platformAdmin := isPlatformAdmin(c)
	if body.Role != nil {
		switch *body.Role {
		case "developer", "tester", "devops":
		case "company_admin":
			if !platformAdmin {
				c.JSON(http.StatusForbidden, gin.H{"error": "only platform admin can assign company_admin role"})
				return
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
	}

	var targetRole, targetPlatformRole string
	err = db.QueryRowContext(c.Request.Context(),
		`SELECT m.role, u.platform_role
		 FROM organization_members m JOIN users u ON u.id=m.user_id
		 WHERE m.organization_id=$1 AND m.user_id=$2`,
		orgID, targetID,
	).Scan(&targetRole, &targetPlatformRole)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization member not found"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	if targetPlatformRole == "platform_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin membership cannot be changed here"})
		return
	}
	if !platformAdmin && targetRole == "company_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only platform admin can modify a company admin"})
		return
	}

	if body.Role != nil && body.CanAccessIncidents != nil {
		_, err = db.ExecContext(c.Request.Context(),
			`UPDATE organization_members SET role=$1, can_access_incidents=$2
			 WHERE organization_id=$3 AND user_id=$4`,
			*body.Role, *body.CanAccessIncidents, orgID, targetID)
	} else if body.Role != nil {
		_, err = db.ExecContext(c.Request.Context(),
			`UPDATE organization_members SET role=$1
			 WHERE organization_id=$2 AND user_id=$3`,
			*body.Role, orgID, targetID)
	} else {
		_, err = db.ExecContext(c.Request.Context(),
			`UPDATE organization_members SET can_access_incidents=$1
			 WHERE organization_id=$2 AND user_id=$3`,
			*body.CanAccessIncidents, orgID, targetID)
	}
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "member updated",
		"user_id": targetID,
	})
}

// DELETE /organizations/:id/members/:userId
func removeOrganizationMember(c *gin.Context) {
	orgID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orgID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}
	targetID, err := strconv.Atoi(c.Param("userId"))
	if err != nil || targetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if !authorizeOrganizationManager(c, orgID) {
		return
	}
	if targetID == currentUserID(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you cannot remove yourself through this endpoint"})
		return
	}

	platformAdmin := isPlatformAdmin(c)
	var targetRole, targetPlatformRole string
	err = db.QueryRowContext(c.Request.Context(),
		`SELECT m.role, u.platform_role
		 FROM organization_members m JOIN users u ON u.id=m.user_id
		 WHERE m.organization_id=$1 AND m.user_id=$2`,
		orgID, targetID,
	).Scan(&targetRole, &targetPlatformRole)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization member not found"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	if targetPlatformRole == "platform_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin cannot be removed"})
		return
	}
	if !platformAdmin && targetRole == "company_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only platform admin can remove a company admin"})
		return
	}

	_, err = db.ExecContext(c.Request.Context(),
		`DELETE FROM organization_members
		 WHERE organization_id=$1 AND user_id=$2`, orgID, targetID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "member removed", "user_id": targetID})
}
