package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Incident represents a DevOps incident.
type Incident struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	ContainerName  string `json:"container_name"`
	Severity      string `json:"severity"`
	Status        string `json:"status"`
	RootCause     string `json:"root_cause"`
	SuggestedFix  string `json:"suggested_fix"`
	Logs          string `json:"logs"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
        TaskID        *int   `json:"task_id,omitempty"`
}

const incidentSelect = `
	SELECT id, title, COALESCE(description,''),
	       COALESCE(container_name,''), severity, status,
	       COALESCE(root_cause,''), COALESCE(suggested_fix,''),
	       COALESCE(logs,''), created_at, updated_at,
               (SELECT id FROM tasks WHERE tasks.incident_id = incidents.id LIMIT 1) AS task_id
	FROM incidents`

const incidentReturning = `
	RETURNING id, title, COALESCE(description,''),
	          COALESCE(container_name,''), severity, status,
	          COALESCE(root_cause,''), COALESCE(suggested_fix,''),
	          COALESCE(logs,''), created_at, updated_at,
               (SELECT id FROM tasks WHERE tasks.incident_id = incidents.id LIMIT 1) AS task_id`

type incidentScanner interface {
	Scan(dest ...interface{}) error
}

func scanIncident(s incidentScanner) (Incident, error) {
	var incident Incident
	var created, updated time.Time

	err := s.Scan(
		&incident.ID,
		&incident.Title,
		&incident.Description,
		&incident.ContainerName,
		&incident.Severity,
		&incident.Status,
		&incident.RootCause,
		&incident.SuggestedFix,
		&incident.Logs,
		&created,
		&updated,
		&incident.TaskID,
	)

	if err != nil {
		return incident, err
	}

	incident.CreatedAt = created.Format(time.RFC3339)
	incident.UpdatedAt = updated.Format(time.RFC3339)

	return incident, nil
}

func validSeverity(value string) bool {
	switch value {
	case "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}

func validIncidentStatus(value string) bool {
	switch value {
	case "open", "acknowledged", "resolved":
		return true
	default:
		return false
	}
}

// POST /incidents
func createIncident(c *gin.Context) {
        var body struct {
                Title         string `json:"title"`
                Description   string `json:"description"`
                ContainerName string `json:"container_name"`
                Severity      string `json:"severity"`
                RootCause     string `json:"root_cause"`
                SuggestedFix  string `json:"suggested_fix"`
                Logs          string `json:"logs"`
        }

        if err := c.ShouldBindJSON(&body); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
                return
        }

        body.Title = strings.TrimSpace(body.Title)
        if body.Title == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
                return
        }

        if body.Severity == "" {
                body.Severity = "medium"
        }
        if !validSeverity(body.Severity) {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid severity"})
                return
        }

        tx, err := db.BeginTx(c.Request.Context(), nil)
        if err != nil {
                fail(c, err)
                return
        }
        defer tx.Rollback()

        incident, err := scanIncident(tx.QueryRow(
                `INSERT INTO incidents
                 (title, description, container_name, severity,
                  root_cause, suggested_fix, logs)
                 VALUES ($1, $2, $3, $4, $5, $6, $7)`+incidentReturning,
                body.Title,
                body.Description,
                body.ContainerName,
                body.Severity,
                body.RootCause,
                body.SuggestedFix,
                body.Logs,
        ))
        if err != nil {
                fail(c, err)
                return
        }

        _, err = tx.ExecContext(
                c.Request.Context(),
                `INSERT INTO incident_events
                 (incident_id, event_type, message, actor_type)
                 VALUES ($1, $2, $3, $4)`,
                incident.ID,
                "incident_created",
                "Incident created",
                "system",
        )
        if err != nil {
                fail(c, err)
                return
        }


        priority := "medium"
        if incident.Severity == "high" || incident.Severity == "critical" {
                priority = "high"
        }

        var taskID int
        err = tx.QueryRowContext(
                c.Request.Context(),
                `INSERT INTO tasks
                 (title, description, project_id, status, priority, incident_id)
                 SELECT $1, $2, id, 'todo', $3, $4
                 FROM projects
                 WHERE id = 1
                 RETURNING id`,
                "Investigate: "+incident.Title,
                incident.Description,
                priority,
                incident.ID,
        ).Scan(&taskID)
        if err != nil {
                fail(c, err)
                return
        }

        incident.TaskID = &taskID

        _, err = tx.ExecContext(
                c.Request.Context(),
                `INSERT INTO incident_events
                 (incident_id, event_type, message, actor_type)
                 VALUES ($1, $2, $3, $4)`,
                incident.ID,
                "task_created",
                "Linked investigation task #"+strconv.Itoa(taskID)+" created",
                "system",
        )
        if err != nil {
                fail(c, err)
                return
        }

if err := tx.Commit(); err != nil {
                fail(c, err)
                return
        }

        c.JSON(http.StatusCreated, incident)
}

// GET /incidents
func listIncidents(c *gin.Context) {
	rows, err := db.Query(incidentSelect + ` ORDER BY created_at DESC`)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	incidents := []Incident{}

	for rows.Next() {
		incident, err := scanIncident(rows)
		if err != nil {
			fail(c, err)
			return
		}
		incidents = append(incidents, incident)
	}

	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"incidents": incidents})
}

// GET /incidents/:id
func getIncident(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid incident id"})
		return
	}

	incident, err := scanIncident(
		db.QueryRow(incidentSelect+` WHERE id = $1`, id),
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, incident)
}

// PATCH /incidents/:id
func updateIncident(c *gin.Context) {
        id, err := strconv.Atoi(c.Param("id"))
        if err != nil || id <= 0 {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid incident id"})
                return
        }

        var patch struct {
                Title        *string `json:"title"`
                Description  *string `json:"description"`
                Severity     *string `json:"severity"`
                Status       *string `json:"status"`
                RootCause    *string `json:"root_cause"`
                SuggestedFix *string `json:"suggested_fix"`
                Logs         *string `json:"logs"`
        }

        if err := c.ShouldBindJSON(&patch); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
                return
        }

        if patch.Title != nil && strings.TrimSpace(*patch.Title) == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "title cannot be empty"})
                return
        }
        if patch.Severity != nil && !validSeverity(*patch.Severity) {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid severity"})
                return
        }
        if patch.Status != nil && !validIncidentStatus(*patch.Status) {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
                return
        }

        sets := []string{}
        args := []interface{}{}

        add := func(column string, value *string) {
                if value != nil {
                        args = append(args, *value)
                        sets = append(sets, column+"=$"+strconv.Itoa(len(args)))
                }
        }

        add("title", patch.Title)
        add("description", patch.Description)
        add("severity", patch.Severity)
        add("status", patch.Status)
        add("root_cause", patch.RootCause)
        add("suggested_fix", patch.SuggestedFix)
        add("logs", patch.Logs)

        if len(sets) == 0 {
                c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields"})
                return
        }

        tx, err := db.BeginTx(c.Request.Context(), nil)
        if err != nil {
                fail(c, err)
                return
        }
        defer tx.Rollback()

        var previousStatus string
        err = tx.QueryRowContext(
                c.Request.Context(),
                `SELECT status FROM incidents WHERE id=$1 FOR UPDATE`,
                id,
        ).Scan(&previousStatus)

        if err == sql.ErrNoRows {
                c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
                return
        }
        if err != nil {
                fail(c, err)
                return
        }

        args = append(args, id)

        query := "UPDATE incidents SET " +
                strings.Join(sets, ", ") +
                " WHERE id=$" + strconv.Itoa(len(args)) +
                incidentReturning

        incident, err := scanIncident(tx.QueryRowContext(
                c.Request.Context(), query, args...,
        ))
        if err != nil {
                fail(c, err)
                return
        }

        if patch.Status != nil && previousStatus != incident.Status {
                message := "Status changed from " + previousStatus +
                        " to " + incident.Status

                _, err = tx.ExecContext(
                        c.Request.Context(),
                        `INSERT INTO incident_events
                         (incident_id, event_type, message, actor_type)
                         VALUES ($1, $2, $3, $4)`,
                        incident.ID,
                        "status_changed",
                        message,
                        "system",
                )
                if err != nil {
                        fail(c, err)
                        return
                }
        }

        if err := tx.Commit(); err != nil {
                fail(c, err)
                return
        }

        c.JSON(http.StatusOK, incident)
}
