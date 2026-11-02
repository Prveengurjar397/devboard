package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type IncidentEvent struct {
	ID         int64  `json:"id"`
	IncidentID int    `json:"incident_id"`
	EventType  string `json:"event_type"`
	Message    string `json:"message"`
	ActorType  string `json:"actor_type"`
	CreatedAt  string `json:"created_at"`
}

// GET /incidents/:id/events
func listIncidentEvents(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid incident id"})
		return
	}


	var exists bool
	err = db.QueryRowContext(c.Request.Context(),
		`SELECT EXISTS (
			SELECT 1 FROM incidents i
			WHERE i.id=$1 AND (
				EXISTS (SELECT 1 FROM users u WHERE u.id=$2 AND u.platform_role='platform_admin')
				OR EXISTS (
					SELECT 1 FROM organization_members m
					WHERE m.user_id=$2 AND m.organization_id=i.organization_id
					  AND (m.role='company_admin' OR m.can_access_incidents=TRUE)
				)
			)
		)`, id, currentUserID(c)).Scan(&exists)
	if err != nil {
		fail(c, err)
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "incident not found"})
		return
	}

	rows, err := db.QueryContext(c.Request.Context(),
		`SELECT id, incident_id, event_type, message, actor_type, created_at
		 FROM incident_events WHERE incident_id=$1
		 ORDER BY created_at ASC, id ASC`, id)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	events := []IncidentEvent{}
	for rows.Next() {
		var event IncidentEvent
		var created time.Time
		if err := rows.Scan(&event.ID, &event.IncidentID, &event.EventType,
			&event.Message, &event.ActorType, &created); err != nil {
			fail(c, err)
			return
		}
		event.CreatedAt = created.Format(time.RFC3339)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}
