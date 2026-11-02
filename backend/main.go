package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

type Task struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ProjectID   int     `json:"project_id"`
	AssigneeID  *int    `json:"assignee_id"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	DueDate     *string `json:"due_date"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     *int   `json:"owner_id"`
	CreatedAt   string `json:"created_at"`
}

func main() {
	dsn := env("POSTGRES_URL", "postgres://devboard:devboard@localhost:5432/devboard?sslmode=disable")

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("[backend] FATAL open db: %v", err)
	}
	db.SetMaxOpenConns(10)

	for i := 0; i < 30; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("[backend] waiting for postgres (%d)…", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("[backend] FATAL ping db: %v", err)
	}
	log.Println("[backend] connected to postgres")

	r := gin.Default()

	// Public routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "backend",
		})
	})

	r.POST("/auth/register", register)
	r.POST("/auth/login", login)

	// Protected routes
	api := r.Group("/")
	api.Use(AuthRequired())

	api.GET("/organizations", listOrganizations)
	api.POST("/organizations", createOrganization)
	api.POST("/organizations/:id/members", addOrganizationMember)
	api.GET("/organizations/:id/members", listOrganizationMembers)
	api.PATCH("/organizations/:id/members/:userId", updateOrganizationMember)
	api.DELETE("/organizations/:id/members/:userId", removeOrganizationMember)

	api.GET("/projects", listProjects)
	api.POST("/projects", createProject)

	api.GET("/tasks", listTasks)
	api.POST("/tasks", createTask)
	api.PATCH("/tasks/:id", updateTask)
	api.GET("/search", searchTasks)

	// Incident routes are authenticated here.
	// Their database ownership enforcement is the next implementation step.
	api.POST("/incidents", createIncident)
	api.GET("/incidents", listIncidents)
	api.GET("/incidents/:id", getIncident)
	api.PATCH("/incidents/:id", updateIncident)
	api.GET("/incidents/:id/events", listIncidentEvents)

	port := env("PORT", "8080")
	log.Printf("[backend] listening on :%s", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[backend] FATAL: %v", err)
	}
}

func listProjects(c *gin.Context) {
	userID := currentUserID(c)

	query := `
		SELECT p.id, p.name, COALESCE(p.description,''), p.owner_id, p.created_at
		FROM projects p
		WHERE EXISTS (
			SELECT 1 FROM users u
			WHERE u.id=$1 AND u.platform_role='platform_admin'
		)
		OR EXISTS (
			SELECT 1 FROM organization_members m
			WHERE m.organization_id=p.organization_id AND m.user_id=$1
		)
		ORDER BY p.id`

	rows, err := db.QueryContext(c.Request.Context(), query, userID)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		var created time.Time
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &created); err != nil {
			fail(c, err)
			return
		}
		p.CreatedAt = created.Format(time.RFC3339)
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func createProject(c *gin.Context) {
	var body struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		OrganizationID int    `json:"organization_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	userID := currentUserID(c)
	isAdmin := isPlatformAdmin(c)
	var orgID int

	if isAdmin {
		if body.OrganizationID > 0 {
			orgID = body.OrganizationID
		} else {
			err := db.QueryRowContext(c.Request.Context(),
				`SELECT id FROM organizations ORDER BY id LIMIT 1`).Scan(&orgID)
			if err != nil {
				fail(c, err)
				return
			}
		}
	} else {
		err := db.QueryRowContext(c.Request.Context(),
			`SELECT organization_id
			 FROM organization_members
			 WHERE user_id=$1
			 ORDER BY organization_id
			 LIMIT 1`, userID).Scan(&orgID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "you must belong to a company before creating a project"})
			return
		}
		if err != nil {
			fail(c, err)
			return
		}
		if body.OrganizationID > 0 && body.OrganizationID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "you cannot create projects in another company"})
			return
		}
	}

	var exists bool
	if err := db.QueryRowContext(c.Request.Context(),
		`SELECT EXISTS(SELECT 1 FROM organizations WHERE id=$1)`, orgID).Scan(&exists); err != nil {
		fail(c, err)
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	var project Project
	var created time.Time
	err := db.QueryRowContext(c.Request.Context(),
		`INSERT INTO projects (name, description, owner_id, organization_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, name, COALESCE(description,''), owner_id, created_at`,
		strings.TrimSpace(body.Name), body.Description, userID, orgID,
	).Scan(&project.ID, &project.Name, &project.Description, &project.OwnerID, &created)
	if err != nil {
		fail(c, err)
		return
	}

	project.CreatedAt = created.Format(time.RFC3339)
	c.JSON(http.StatusCreated, project)
}

func listTasks(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Query("project_id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}

	userID := currentUserID(c)
	query := `
		SELECT t.id, t.title, COALESCE(t.description,''),
		       t.project_id, t.assignee_id, t.status, t.priority,
		       t.due_date, t.created_at, t.updated_at
		FROM tasks t
		JOIN projects p ON p.id=t.project_id
		WHERE t.project_id=$1
		  AND t.incident_id IS NULL
		  AND (
			EXISTS (SELECT 1 FROM users u WHERE u.id=$2 AND u.platform_role='platform_admin')
			OR EXISTS (
				SELECT 1 FROM organization_members m
				WHERE m.organization_id=p.organization_id AND m.user_id=$2
			)
		  )
		ORDER BY t.id`

	rows, err := db.QueryContext(c.Request.Context(), query, projectID, userID)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	tasks, err := scanTasks(rows)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "source": "database"})
}

func createTask(c *gin.Context) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		ProjectID   int    `json:"project_id"`
		Status      string `json:"status"`
		Priority    string `json:"priority"`
	}

	if err := c.ShouldBindJSON(&body); err != nil ||
		strings.TrimSpace(body.Title) == "" || body.ProjectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and project_id are required"})
		return
	}

	if body.Status == "" {
		body.Status = "todo"
	}
	if body.Priority == "" {
		body.Priority = "medium"
	}
	if body.Status != "todo" && body.Status != "in_progress" &&
		body.Status != "blocked" && body.Status != "done" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task status"})
		return
	}
	if body.Priority != "low" && body.Priority != "medium" && body.Priority != "high" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task priority"})
		return
	}

	userID := currentUserID(c)
	query := `
		INSERT INTO tasks (title, description, project_id, status, priority, created_by)
		SELECT $1, $2, p.id, $4, $5, $6
		FROM projects p
		WHERE p.id=$3
		  AND (
			EXISTS (SELECT 1 FROM users u WHERE u.id=$6 AND u.platform_role='platform_admin')
			OR EXISTS (
				SELECT 1 FROM organization_members m
				WHERE m.organization_id=p.organization_id AND m.user_id=$6
			)
		  )
		RETURNING id, title, COALESCE(description,''), project_id,
		          assignee_id, status, priority, due_date, created_at, updated_at`

	task, err := scanTask(db.QueryRowContext(
		c.Request.Context(), query,
		strings.TrimSpace(body.Title), body.Description, body.ProjectID,
		body.Status, body.Priority, userID,
	))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found or access denied"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, task)
}

func updateTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var patch map[string]interface{}
	if err := c.ShouldBindJSON(&patch); err != nil || len(patch) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	for key := range patch {
		switch key {
		case "title", "description", "status", "priority":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported field: " + key})
			return
		}
	}

	if v, ok := patch["status"]; ok {
		status, valid := v.(string)
		if !valid || (status != "todo" && status != "in_progress" &&
			status != "blocked" && status != "done") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task status"})
			return
		}
	}
	if v, ok := patch["priority"]; ok {
		priority, valid := v.(string)
		if !valid || (priority != "low" && priority != "medium" && priority != "high") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task priority"})
			return
		}
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		fail(c, err)
		return
	}
	defer tx.Rollback()

	userID := currentUserID(c)

	var createdBy, incidentID sql.NullInt64
	var orgID int
	var role sql.NullString
	var canAccessIncidents, platformAdmin bool

	err = tx.QueryRowContext(c.Request.Context(), `
		SELECT t.created_by, t.incident_id, p.organization_id,
		       m.role, COALESCE(m.can_access_incidents,FALSE),
		       (u.platform_role='platform_admin')
		FROM tasks t
		JOIN projects p ON p.id=t.project_id
		JOIN users u ON u.id=$2
		LEFT JOIN organization_members m
		  ON m.organization_id=p.organization_id AND m.user_id=$2
		WHERE t.id=$1
		  AND (u.platform_role='platform_admin' OR m.user_id IS NOT NULL)
		FOR UPDATE OF t`,
		id, userID,
	).Scan(&createdBy, &incidentID, &orgID, &role, &canAccessIncidents, &platformAdmin)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}

	companyAdmin := role.Valid && role.String == "company_admin"

	if incidentID.Valid {
		if !platformAdmin && !companyAdmin && !canAccessIncidents {
			c.JSON(http.StatusForbidden, gin.H{"error": "incident access permission required"})
			return
		}
		if len(patch) != 1 {
			if _, ok := patch["status"]; !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "incident-linked tasks can only update status"})
				return
			}
			c.JSON(http.StatusForbidden, gin.H{"error": "incident-linked tasks can only update status"})
			return
		}
	} else if !platformAdmin && !companyAdmin &&
		(!createdBy.Valid || int(createdBy.Int64) != userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the task creator or company owner can update this task"})
		return
	}

	keys := []string{}
	args := []interface{}{}
	for _, key := range []string{"title", "description", "status", "priority"} {
		if value, ok := patch[key]; ok {
			args = append(args, value)
			keys = append(keys, key+"=$"+strconv.Itoa(len(args)))
		}
	}
	if len(keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updatable fields"})
		return
	}

	args = append(args, id)
	query := "UPDATE tasks SET " + join(keys, ", ") +
		" WHERE id=$" + strconv.Itoa(len(args)) + taskReturning

	task, err := scanTask(tx.QueryRowContext(c.Request.Context(), query, args...))
	if err != nil {
		fail(c, err)
		return
	}

	if incidentID.Valid {
		var nextStatus string
		switch task.Status {
		case "todo":
			nextStatus = "open"
		case "in_progress":
			nextStatus = "in_progress"
		case "blocked":
			nextStatus = "acknowledged"
		case "done":
			nextStatus = "resolved"
		}

		var previousStatus string
		err = tx.QueryRowContext(c.Request.Context(),
			`UPDATE incidents SET status=$1
			 WHERE id=$2 AND organization_id=$3 AND status<>$1
			 RETURNING status`,
			nextStatus, incidentID.Int64, orgID,
		).Scan(&previousStatus)

		if err != nil && err != sql.ErrNoRows {
			fail(c, err)
			return
		}
		if err == nil {
			_, err = tx.ExecContext(c.Request.Context(),
				`INSERT INTO incident_events
				 (incident_id,event_type,message,actor_type)
				 VALUES($1,'task_status_sync',$2,'system')`,
				incidentID.Int64,
				"Linked task status changed to "+task.Status+
					"; incident status changed to "+previousStatus,
			)
			if err != nil {
				fail(c, err)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

func searchTasks(c *gin.Context) {
	q := c.Query("q")
	projectID, err := strconv.Atoi(c.Query("project_id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}

	userID := currentUserID(c)
	query := `
		SELECT t.id, t.title, COALESCE(t.description,''),
		       t.project_id, t.assignee_id, t.status, t.priority,
		       t.due_date, t.created_at, t.updated_at
		FROM tasks t
		JOIN projects p ON p.id=t.project_id
		WHERE t.project_id=$1
		  AND t.incident_id IS NULL
		  AND t.title ILIKE '%' || $3 || '%'
		  AND (
			EXISTS (SELECT 1 FROM users u WHERE u.id=$2 AND u.platform_role='platform_admin')
			OR EXISTS (
				SELECT 1 FROM organization_members m
				WHERE m.organization_id=p.organization_id AND m.user_id=$2
			)
		  )
		ORDER BY t.id`

	rows, err := db.QueryContext(c.Request.Context(), query, projectID, userID, q)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	tasks, err := scanTasks(rows)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": tasks})
}

const taskSelect = `SELECT id, title, COALESCE(description,''), project_id,
	assignee_id, status, priority, due_date, created_at, updated_at FROM tasks`

const taskReturning = ` RETURNING id, title, COALESCE(description,''), project_id,
	assignee_id, status, priority, due_date, created_at, updated_at`

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanTask(s scannable) (Task, error) {
	var t Task
	var due sql.NullTime
	var created, updated time.Time

	err := s.Scan(
		&t.ID, &t.Title, &t.Description, &t.ProjectID,
		&t.AssigneeID, &t.Status, &t.Priority, &due,
		&created, &updated,
	)
	if err != nil {
		return t, err
	}

	if due.Valid {
		value := due.Time.Format("2006-01-02")
		t.DueDate = &value
	}

	t.CreatedAt = created.Format(time.RFC3339)
	t.UpdatedAt = updated.Format(time.RFC3339)

	return t, nil
}

func scanTasks(rows *sql.Rows) ([]Task, error) {
	tasks := []Task{}

	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}

	return tasks, rows.Err()
}

func join(parts []string, sep string) string {
	out := ""

	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}

	return out
}

func fail(c *gin.Context, err error) {
	log.Printf("[backend] ERROR: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
