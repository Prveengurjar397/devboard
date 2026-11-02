package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Incident struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	ContainerName string `json:"container_name"`
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
	SELECT id, title, COALESCE(description,''), COALESCE(container_name,''),
	       severity, status, COALESCE(root_cause,''), COALESCE(suggested_fix,''),
	       COALESCE(logs,''), created_at, updated_at,
	       (SELECT id FROM tasks WHERE tasks.incident_id = incidents.id LIMIT 1)
	FROM incidents`

const incidentReturning = `
	RETURNING id, title, COALESCE(description,''), COALESCE(container_name,''),
	          severity, status, COALESCE(root_cause,''), COALESCE(suggested_fix,''),
	          COALESCE(logs,''), created_at, updated_at,
	          (SELECT id FROM tasks WHERE tasks.incident_id = incidents.id LIMIT 1)`

type incidentScanner interface {
	Scan(dest ...interface{}) error
}

func scanIncident(s incidentScanner) (Incident, error) {
	var incident Incident
	var created, updated time.Time
	err := s.Scan(
		&incident.ID, &incident.Title, &incident.Description,
		&incident.ContainerName, &incident.Severity, &incident.Status,
		&incident.RootCause, &incident.SuggestedFix, &incident.Logs,
		&created, &updated, &incident.TaskID,
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
	case "open", "acknowledged", "in_progress", "resolved":
		return true
	default:
		return false
	}
}

func taskStatusForIncident(status string) string {
	switch status {
	case "open":
		return "todo"
	case "in_progress":
		return "in_progress"
	case "acknowledged":
		return "blocked"
	case "resolved":
		return "done"
	default:
		return ""
	}
}

// Resolve a single incident-enabled organization for a regular member.
// Platform admins must explicitly choose an organization.
func resolveIncidentOrganization(c *gin.Context, requestedOrgID int) (int, bool) {
	userID := currentUserID(c)

	if isPlatformAdmin(c) {
		if requestedOrgID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id is required for platform admin"})
			return 0, false
		}
		return requestedOrgID, true
	}

	rows, err := db.QueryContext(c.Request.Context(),
		`SELECT organization_id FROM organization_members
		 WHERE user_id=$1 AND (role='company_admin' OR can_access_incidents=TRUE)
		 ORDER BY organization_id LIMIT 2`, userID)
	if err != nil {
		fail(c, err)
		return 0, false
	}
	defer rows.Close()

	var orgIDs []int
	for rows.Next() {
		var orgID int
		if err := rows.Scan(&orgID); err != nil {
			fail(c, err)
			return 0, false
		}
		orgIDs = append(orgIDs, orgID)
	}
	if err := rows.Err(); err != nil {
		fail(c, err)
		return 0, false
	}

	if requestedOrgID > 0 {
		for _, orgID := range orgIDs {
			if orgID == requestedOrgID {
				return orgID, true
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "no incident permission for this organization"})
		return 0, false
	}

	if len(orgIDs) == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "incident access is not enabled for your account"})
		return 0, false
	}
	if len(orgIDs) > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id is required when you belong to multiple organizations"})
		return 0, false
	}
	return orgIDs[0], true
}

// POST /incidents
func createIncident(c *gin.Context) {
	userID := currentUserID(c)

	var body struct {
		OrganizationID int    `json:"organization_id"`
		Title           string `json:"title"`
		Description     string `json:"description"`
		ContainerName   string `json:"container_name"`
		Severity        string `json:"severity"`
		RootCause       string `json:"root_cause"`
		SuggestedFix    string `json:"suggested_fix"`
		Logs            string `json:"logs"`
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

	orgID, ok := resolveIncidentOrganization(c, body.OrganizationID)
	if !ok {
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		fail(c, err)
		return
	}
	defer tx.Rollback()

	var organizationExists bool
	if err := tx.QueryRowContext(c.Request.Context(),
		`SELECT EXISTS(SELECT 1 FROM organizations WHERE id=$1)`, orgID,
	).Scan(&organizationExists); err != nil {
		fail(c, err)
		return
	}
	if !organizationExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	incident, err := scanIncident(tx.QueryRowContext(c.Request.Context(),
		`INSERT INTO incidents
		 (title, description, container_name, severity, root_cause,
		  suggested_fix, logs, owner_id, organization_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`+incidentReturning,
		body.Title, body.Description, body.ContainerName, body.Severity,
		body.RootCause, body.SuggestedFix, body.Logs, userID, orgID))
	if err != nil {
		fail(c, err)
		return
	}

	if _, err = tx.ExecContext(c.Request.Context(),
		`INSERT INTO incident_events (incident_id,event_type,message,actor_type)
		 VALUES ($1,'incident_created','Incident created','user')`, incident.ID); err != nil {
		fail(c, err)
		return
	}

	priority := "medium"
	if incident.Severity == "high" || incident.Severity == "critical" {
		priority = "high"
	}

	var taskID int
	err = tx.QueryRowContext(c.Request.Context(),
		`INSERT INTO tasks
		 (title, description, project_id, status, priority, incident_id, created_by)
		 SELECT $1,$2,p.id,'todo',$3,$4,$5
		 FROM projects p
		 WHERE p.organization_id=$6
		 ORDER BY p.id
		 LIMIT 1
		 RETURNING id`,
		"Investigate: "+incident.Title, incident.Description,
		priority, incident.ID, userID, orgID).Scan(&taskID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusConflict, gin.H{"error": "create a project in this organization before creating an incident"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}

	incident.TaskID = &taskID
	if _, err = tx.ExecContext(c.Request.Context(),
		`INSERT INTO incident_events (incident_id,event_type,message,actor_type)
		 VALUES ($1,'task_created',$2,'system')`,
		incident.ID, "Linked investigation task #"+strconv.Itoa(taskID)+" created"); err != nil {
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
	userID := currentUserID(c)
	rows, err := db.QueryContext(c.Request.Context(),
		incidentSelect+`
		WHERE EXISTS (
			SELECT 1 FROM users u WHERE u.id=$1 AND u.platform_role='platform_admin'
		) OR EXISTS (
			SELECT 1 FROM organization_members m
			WHERE m.user_id=$1 AND m.organization_id=incidents.organization_id
			  AND (m.role='company_admin' OR m.can_access_incidents=TRUE)
		)
		ORDER BY created_at DESC`, userID)
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

	incident, err := scanIncident(db.QueryRowContext(c.Request.Context(),
		incidentSelect+` WHERE id=$1 AND (
			EXISTS (SELECT 1 FROM users u WHERE u.id=$2 AND u.platform_role='platform_admin')
			OR EXISTS (
				SELECT 1 FROM organization_members m
				WHERE m.user_id=$2 AND m.organization_id=incidents.organization_id
				  AND (m.role='company_admin' OR m.can_access_incidents=TRUE)
			)
		)`, id, currentUserID(c)))

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
	userID := currentUserID(c)
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
	var orgID int
	err = tx.QueryRowContext(c.Request.Context(),
		`SELECT status, organization_id FROM incidents WHERE id=$1 FOR UPDATE`, id,
	).Scan(&previousStatus, &orgID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}

	allowed, err := canAccessIncidentOrg(userID, orgID)
	if err != nil {
		fail(c, err)
		return
	}
	if !allowed {
		c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
		return
	}

	args = append(args, id)
	query := "UPDATE incidents SET "+strings.Join(sets, ", ")+
		" WHERE id=$"+strconv.Itoa(len(args))+incidentReturning
	incident, err := scanIncident(tx.QueryRowContext(c.Request.Context(), query, args...))
	if err != nil {
		fail(c, err)
		return
	}

	if patch.Status != nil && previousStatus != incident.Status {
		message := "Status changed from "+previousStatus+" to "+incident.Status
		if _, err = tx.ExecContext(c.Request.Context(),
			`INSERT INTO incident_events (incident_id,event_type,message,actor_type)
			 VALUES ($1,'status_changed',$2,'user')`, incident.ID, message); err != nil {
			fail(c, err)
			return
		}

		taskStatus := taskStatusForIncident(incident.Status)
		if taskStatus != "" {
			result, updateErr := tx.ExecContext(c.Request.Context(),
				`UPDATE tasks SET status=$1, updated_at=NOW()
				 WHERE incident_id=$2 AND status<>$1`, taskStatus, incident.ID)
			if updateErr != nil {
				fail(c, updateErr)
				return
			}
			if count, _ := result.RowsAffected(); count > 0 {
				if _, err = tx.ExecContext(c.Request.Context(),
					`INSERT INTO incident_events (incident_id,event_type,message,actor_type)
					 VALUES ($1,'task_status_synced',$2,'system')`,
					incident.ID, "Linked task status synchronized to "+taskStatus); err != nil {
					fail(c, err)
					return
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, incident)
}
