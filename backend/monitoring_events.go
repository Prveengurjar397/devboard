package main

import (
	"strconv"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ingestMonitoringEvent(c *gin.Context) {
	var body struct {
		Title         string `json:"title"`
		Description   string `json:"description"`
		ContainerName string `json:"container_name"`
		Severity      string `json:"severity"`
		RootCause     string `json:"root_cause"`
		SuggestedFix  string `json:"suggested_fix"`
		Logs          string `json:"logs"`
		Fingerprint   string `json:"fingerprint"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	body.Title = strings.TrimSpace(body.Title)
	body.ContainerName = strings.TrimSpace(body.ContainerName)
	body.Fingerprint = strings.TrimSpace(body.Fingerprint)

	if body.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	if len(body.Title) > 200 || len(body.ContainerName) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title or container name is too long"})
		return
	}
	if len(body.Description) > 5000 || len(body.RootCause) > 5000 ||
		len(body.SuggestedFix) > 5000 || len(body.Logs) > 20000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event content is too long"})
		return
	}

	if body.Severity == "" {
		body.Severity = "medium"
	}
	if !validSeverity(body.Severity) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid severity"})
		return
	}

	sourceValue, sourceOK := c.Get("monitoring_source_id")
	orgValue, orgOK := c.Get("monitoring_organization_id")
	if !sourceOK || !orgOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "monitoring authentication required"})
		return
	}

	sourceID, ok1 := sourceValue.(int64)
	orgID, ok2 := orgValue.(int)
	if !ok1 || !ok2 || sourceID <= 0 || orgID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid monitoring context"})
		return
	}

	if body.Fingerprint == "" {
		sum := sha256.Sum256([]byte(
			strings.Join([]string{
				body.ContainerName,
				body.Title,
				body.Description,
			}, "|"),
		))
		body.Fingerprint = hex.EncodeToString(sum[:])
	} else if len(body.Fingerprint) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fingerprint is too long"})
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		fail(c, err)
		return
	}
	defer tx.Rollback()

	// Verify source and organization again inside the transaction.
	var verifiedOrgID int
	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT ms.organization_id
		FROM monitoring_sources ms
		JOIN organizations o ON o.id = ms.organization_id
		WHERE ms.id = $1
		  AND ms.organization_id = $2
		  AND ms.is_enabled = TRUE
		  AND ms.token_revoked_at IS NULL
		  AND o.is_active = TRUE
		  AND o.deleted_at IS NULL
		FOR UPDATE OF ms
	`, sourceID, orgID).Scan(&verifiedOrgID)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "monitoring source is no longer active"})
		return
	}

	// Serialize events with the same organization and fingerprint.
	if _, err = tx.ExecContext(c.Request.Context(),
		`SELECT pg_advisory_xact_lock($1, hashtext($2))`,
		orgID, body.Fingerprint,
	); err != nil {
		fail(c, err)
		return
	}

	var incidentID int
	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT id
		FROM incidents
		WHERE organization_id = $1
		  AND fingerprint = $2
		  AND status <> 'resolved'
		ORDER BY id DESC
		LIMIT 1
		FOR UPDATE
	`, orgID, body.Fingerprint).Scan(&incidentID)

	if err == nil {
		// Existing unresolved incident: update it, do not create another task.
		_, err = tx.ExecContext(c.Request.Context(), `
			UPDATE incidents
			SET last_seen_at = NOW(),
			    occurrence_count = occurrence_count + 1,
			    severity = $2,
			    logs = CASE WHEN $3 = '' THEN logs ELSE $3 END,
			    root_cause = CASE WHEN $4 = '' THEN root_cause ELSE $4 END,
			    suggested_fix = CASE WHEN $5 = '' THEN suggested_fix ELSE $5 END,
			    updated_at = NOW()
			WHERE id = $1
		`, incidentID, body.Severity, body.Logs, body.RootCause, body.SuggestedFix)
		if err != nil {
			fail(c, err)
			return
		}

		_, err = tx.ExecContext(c.Request.Context(), `
			INSERT INTO incident_events (incident_id, event_type, message, actor_type)
			VALUES ($1, 'monitoring_event_repeat', $2, 'monitor')
		`, incidentID, "Monitoring detected this issue again")
		if err != nil {
			fail(c, err)
			return
		}

		var taskID *int
		var existingTaskID int
		err = tx.QueryRowContext(c.Request.Context(),
			`SELECT id FROM tasks WHERE incident_id = $1`, incidentID,
		).Scan(&existingTaskID)
		if err == nil {
			taskID = &existingTaskID
		}

		if err != nil && err != sql.ErrNoRows {
			fail(c, err)
			return
		}

		if err = tx.Commit(); err != nil {
			fail(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":     "existing incident updated",
			"incident_id": incidentID,
			"task_id":     taskID,
			"duplicate":   true,
		})
		return
	}

	if err != sql.ErrNoRows {
		fail(c, err)
		return
	}

	// No unresolved incident exists: create a new one.
	err = tx.QueryRowContext(c.Request.Context(), `
		INSERT INTO incidents
			(title, description, container_name, severity, status,
			 root_cause, suggested_fix, logs, owner_id, organization_id,
			 source, fingerprint, last_seen_at, occurrence_count)
		VALUES
			($1, $2, $3, $4, 'open',
			 $5, $6, $7, NULL, $8,
			 'docker', $9, NOW(), 1)
		RETURNING id
	`, body.Title, body.Description, body.ContainerName, body.Severity,
		body.RootCause, body.SuggestedFix, body.Logs, orgID, body.Fingerprint,
	).Scan(&incidentID)
	if err != nil {
		fail(c, err)
		return
	}

	_, err = tx.ExecContext(c.Request.Context(), `
		INSERT INTO incident_events (incident_id, event_type, message, actor_type)
		VALUES ($1, 'incident_created', 'Incident created by container monitoring', 'monitor')
	`, incidentID)
	if err != nil {
		fail(c, err)
		return
	}

	priority := "medium"
	if body.Severity == "high" || body.Severity == "critical" {
		priority = "high"
	}

	var taskID int
	err = tx.QueryRowContext(c.Request.Context(), `
		INSERT INTO tasks
			(title, description, project_id, status, priority,
			 incident_id, created_by, is_automated)
		SELECT $1, $2, p.id, 'todo', $3, $4, NULL, TRUE
		FROM projects p
		WHERE p.organization_id = $5
		ORDER BY p.id
		LIMIT 1
		RETURNING id
	`, "Investigate: "+body.Title, body.Description,
		priority, incidentID, orgID,
	).Scan(&taskID)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusConflict, gin.H{
				"error": "create a project in this organization before monitoring can create incidents",
			})
			return
		}
		fail(c, err)
		return
	}

	_, err = tx.ExecContext(c.Request.Context(), `
		INSERT INTO incident_events (incident_id, event_type, message, actor_type)
		VALUES ($1, 'task_created', $2, 'system')
	`, incidentID, "Linked automated investigation task #"+hex.EncodeToString([]byte{
		byte(taskID >> 24), byte(taskID >> 16), byte(taskID >> 8), byte(taskID),
	}))
	if err != nil {
		fail(c, err)
		return
	}

	if err = tx.Commit(); err != nil {
		fail(c, err)
		return
	}

	// Notify organization members who can access incidents.
	var organizationName string
	_ = db.QueryRowContext(c.Request.Context(),
		`SELECT name FROM organizations WHERE id=$1`, orgID,
	).Scan(&organizationName)

	rows, notifyErr := db.QueryContext(c.Request.Context(), `
		SELECT DISTINCT u.email
		FROM organization_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1
		  AND (m.role = 'company_admin' OR m.can_access_incidents = TRUE)
		  AND u.email <> ''
	`, orgID)

	emailsSent := 0
	emailErrors := 0

	if notifyErr == nil {
		var recipients []string
		for rows.Next() {
			var recipient string
			if scanErr := rows.Scan(&recipient); scanErr != nil {
				emailErrors++
				continue
			}
			recipients = append(recipients, recipient)
		}
		if rows.Err() != nil {
			emailErrors++
		}
		rows.Close()

		mailBody := "A new monitoring incident was detected in OpsMind AI.\n\n" +
			"Organization: " + organizationName + "\n" +
			"Incident ID: " + strconv.Itoa(incidentID) + "\n" +
			"Task ID: " + strconv.Itoa(taskID) + "\n" +
			"Title: " + body.Title + "\n" +
			"Container: " + body.ContainerName + "\n" +
			"Severity: " + body.Severity + "\n\n" +
			"Please open OpsMind AI to investigate this incident."

		for _, recipient := range recipients {
			if err := sendEmail(recipient, "OpsMind AI: New incident detected", mailBody); err != nil {
				emailErrors++
				continue
			}
			emailsSent++
		}
	} else {
		emailErrors++
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":      "monitoring incident created",
		"incident_id":  incidentID,
		"task_id":      taskID,
		"duplicate":    false,
		"emails_sent":  emailsSent,
		"email_errors": emailErrors,
	})
}
