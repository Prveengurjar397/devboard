package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func createOrganizationInvitation(c *gin.Context) {
	orgID := 0
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &orgID); err != nil || orgID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	if !authorizeOrganizationManager(c, orgID) {
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
	if len(body.Email) > 255 || !strings.Contains(body.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required"})
		return
	}

	switch body.Role {
	case "developer", "tester", "devops":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}

	var targetPlatformRole string
	err := db.QueryRowContext(c.Request.Context(),
		`SELECT platform_role FROM users
		 WHERE lower(email)=$1 AND is_legacy=FALSE`,
		body.Email,
	).Scan(&targetPlatformRole)

	if err == nil && targetPlatformRole == "platform_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "platform admin cannot be invited"})
		return
	}
	if err != nil && err != sql.ErrNoRows {
		fail(c, err)
		return
	}

	randomToken := make([]byte, 32)
	if _, err := rand.Read(randomToken); err != nil {
		fail(c, err)
		return
	}

	token := hex.EncodeToString(randomToken)
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var invitationID int64
	err = db.QueryRowContext(c.Request.Context(),
		`INSERT INTO organization_invitations
		 (organization_id,email,role,can_access_incidents,token_hash,expires_at,created_by)
		 VALUES($1,$2,$3,$4,$5,NOW() + INTERVAL '48 hours',$6)
		 RETURNING id`,
		orgID, body.Email, body.Role, body.CanAccessIncidents,
		tokenHash, currentUserID(c),
	).Scan(&invitationID)

	if err != nil {
		fail(c, err)
		return
	}

	baseURL := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	inviteURL := baseURL + "/invite/accept?token=" + token

	var organizationName string
	if err := db.QueryRowContext(c.Request.Context(),
		`SELECT name FROM organizations WHERE id=$1`, orgID,
	).Scan(&organizationName); err != nil {
		organizationName = "your organization"
	}

	mailBody := fmt.Sprintf(
		"Hello,\n\nYou have been invited to join %s on OpsMind AI.\n\nRole: %s\nIncident access: %t\n\nAccept your invitation using this link:\n%s\n\nThis invitation expires in 48 hours.\n\nOpsMind AI",
		organizationName, body.Role, body.CanAccessIncidents, inviteURL,
	)

	emailErr := sendEmail(
		body.Email,
		"Your OpsMind AI organization invitation",
		mailBody,
	)
	if emailErr != nil {
    log.Printf("INVITATION EMAIL FAILED: %v", emailErr)
} else {
    log.Println("INVITATION EMAIL SENT SUCCESSFULLY")
}

	c.JSON(http.StatusCreated, gin.H{
		"message": "invitation created",
		"invitation_id": invitationID,
		"email": body.Email,
		"role": body.Role,
		"can_access_incidents": body.CanAccessIncidents,
		"expires_in_hours": 48,
		"invite_url": inviteURL,
		"email_sent": emailErr == nil,
	})
}

func acceptOrganizationInvitation(c *gin.Context) {
	var body struct {
		Token    string `json:"token"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	body.Token = strings.TrimSpace(body.Token)
	body.Name = strings.TrimSpace(body.Name)

	if len(body.Token) != 64 || len(body.Password) < 8 || len(body.Password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid invitation token and password (8-72 bytes) are required"})
		return
	}

	if jwtSecret() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication is not configured"})
		return
	}

	hash := sha256.Sum256([]byte(body.Token))
	tokenHash := hex.EncodeToString(hash[:])

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		fail(c, err)
		return
	}
	defer tx.Rollback()

	var invitationID int64
	var orgID int
	var email, role string
	var incidentAccess bool
	var expiresAt time.Time

	err = tx.QueryRowContext(c.Request.Context(),
		`SELECT id,organization_id,email,role,can_access_incidents,expires_at
		 FROM organization_invitations
		 WHERE token_hash=$1 AND accepted_at IS NULL
		 FOR UPDATE`,
		tokenHash,
	).Scan(&invitationID, &orgID, &email, &role, &incidentAccess, &expiresAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invitation is invalid or already used"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	if !expiresAt.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invitation has expired"})
		return
	}

	var userID int
	var name, passwordHash string

	err = tx.QueryRowContext(c.Request.Context(),
		`SELECT id,name,password_hash FROM users
		 WHERE lower(email)=$1 AND is_legacy=FALSE
		 FOR UPDATE`,
		email,
	).Scan(&userID, &name, &passwordHash)

	if err == sql.ErrNoRows {
		if body.Name == "" || len(body.Name) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required for a new account"})
			return
		}

		newHash, hashErr := bcrypt.GenerateFromPassword(
			[]byte(body.Password), bcrypt.DefaultCost,
		)
		if hashErr != nil {
			fail(c, hashErr)
			return
		}

		err = tx.QueryRowContext(c.Request.Context(),
			`INSERT INTO users(name,email,password_hash,platform_role,is_legacy)
			 VALUES($1,$2,$3,'user',FALSE) RETURNING id`,
			body.Name, email, string(newHash),
		).Scan(&userID)

		if err != nil {
			fail(c, err)
			return
		}
		name = body.Name
	} else if err != nil {
		fail(c, err)
		return
	} else {
		if bcrypt.CompareHashAndPassword(
			[]byte(passwordHash), []byte(body.Password),
		) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "existing account password is incorrect",
			})
			return
		}
	}

	_, err = tx.ExecContext(c.Request.Context(),
		`INSERT INTO organization_members
		 (organization_id,user_id,role,can_access_incidents)
		 VALUES($1,$2,$3,$4)
		 ON CONFLICT(organization_id,user_id)
		 DO UPDATE SET role=EXCLUDED.role,
		               can_access_incidents=EXCLUDED.can_access_incidents`,
		orgID, userID, role, incidentAccess,
	)
	if err != nil {
		fail(c, err)
		return
	}

	_, err = tx.ExecContext(c.Request.Context(),
		`UPDATE organization_invitations
		 SET accepted_at=NOW()
		 WHERE id=$1 AND accepted_at IS NULL`,
		invitationID,
	)
	if err != nil {
		fail(c, err)
		return
	}

	if err := tx.Commit(); err != nil {
		fail(c, err)
		return
	}

	issueAuthToken(c, userID, name, email)
}
