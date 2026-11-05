package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type chatbotContextData struct {
	ProjectCount        int
	OpenTaskCount       int
	InProgressTaskCount int
	OpenIncidentCount   int
	CriticalIncidents   int
	RecentIncidents     []string
}

func loadChatbotContext(ctx context.Context, organizationID int) (chatbotContextData, error) {
	var data chatbotContextData

	err := db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM projects WHERE organization_id = $1),
			(SELECT COUNT(*) FROM tasks
			 WHERE project_id IN (SELECT id FROM projects WHERE organization_id = $1)
			   AND incident_id IS NULL AND status = 'todo'),
			(SELECT COUNT(*) FROM tasks
			 WHERE project_id IN (SELECT id FROM projects WHERE organization_id = $1)
			   AND incident_id IS NULL AND status = 'in_progress'),
			(SELECT COUNT(*) FROM incidents
			 WHERE organization_id = $1 AND status <> 'resolved'),
			(SELECT COUNT(*) FROM incidents
			 WHERE organization_id = $1 AND status <> 'resolved'
			   AND severity = 'critical')
	`, organizationID).Scan(
		&data.ProjectCount,
		&data.OpenTaskCount,
		&data.InProgressTaskCount,
		&data.OpenIncidentCount,
		&data.CriticalIncidents,
	)
	if err != nil {
		return data, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT title, severity, status, container_name
		FROM incidents
		WHERE organization_id = $1 AND status <> 'resolved'
		ORDER BY
			CASE severity
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				ELSE 4
			END,
			updated_at DESC
		LIMIT 8
	`, organizationID)
	if err != nil {
		return data, err
	}
	defer rows.Close()

	data.RecentIncidents = make([]string, 0)

	for rows.Next() {
		var title, severity, status string
		var container sql.NullString

		if err := rows.Scan(&title, &severity, &status, &container); err != nil {
			return data, err
		}

		item := fmt.Sprintf(
			"Incident: %s | Severity: %s | Status: %s",
			title, severity, status,
		)
		if container.Valid && container.String != "" {
			item += " | Container: " + container.String
		}
		data.RecentIncidents = append(data.RecentIncidents, item)
	}

	return data, rows.Err()
}

func (data chatbotContextData) asPromptContext() string {
	var builder strings.Builder

	fmt.Fprintf(&builder,
		"Company workspace summary:\nProjects: %d\nManual tasks todo: %d\nManual tasks in progress: %d\nOpen incidents: %d\nCritical open incidents: %d\n",
		data.ProjectCount,
		data.OpenTaskCount,
		data.InProgressTaskCount,
		data.OpenIncidentCount,
		data.CriticalIncidents,
	)

	builder.WriteString("\nRecent unresolved incidents:\n")
	if len(data.RecentIncidents) == 0 {
		builder.WriteString("No unresolved incidents found.\n")
	} else {
		for _, incident := range data.RecentIncidents {
			builder.WriteString("- ")
			builder.WriteString(incident)
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

func runOllama(ctx context.Context, prompt string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OLLAMA_URL")), "/")
	if baseURL == "" {
		baseURL = "http://ollama:11434"
	}

	model := strings.TrimSpace(os.Getenv("OLLAMA_MODEL"))
	if model == "" {
		model = "llama3.2:1b"
	}

	requestBody := map[string]interface{}{
                "model":  model,
                "prompt": prompt,
                "stream": false,
                "options": map[string]interface{}{
                        "num_predict": 256,
                        "temperature": 0.3,
                },
        }

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseURL+"/api/generate",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 38 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned HTTP %d", response.StatusCode)
	}

	var result struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("ollama error: %s", result.Error)
	}

	answer := strings.TrimSpace(result.Response)
	if answer == "" {
		return "", fmt.Errorf("ollama returned an empty response")
	}

	return answer, nil
}

func basicChatbotAnswer(message string, data chatbotContextData) string {
	lower := strings.ToLower(message)

	if strings.Contains(lower, "incident") || strings.Contains(lower, "alert") {
		if data.OpenIncidentCount == 0 {
			return "There are currently no unresolved incidents in this workspace."
		}
		return fmt.Sprintf(
			"This workspace has %d unresolved incidents, including %d critical incidents. Review the Incidents page for details and status updates.",
			data.OpenIncidentCount,
			data.CriticalIncidents,
		)
	}

	if strings.Contains(lower, "task") {
		return fmt.Sprintf(
			"This workspace has %d manual tasks in todo status and %d manual tasks in progress. Open the Tasks or Dashboard section to review them.",
			data.OpenTaskCount,
			data.InProgressTaskCount,
		)
	}

	if strings.Contains(lower, "project") {
		return fmt.Sprintf(
			"This workspace currently has %d projects. Open the Projects section to review project details.",
			data.ProjectCount,
		)
	}

	return fmt.Sprintf(
		"Here is the current workspace summary: %d projects, %d manual tasks todo, %d manual tasks in progress, and %d unresolved incidents (%d critical). Configure Ollama to enable AI-generated responses.",
		data.ProjectCount,
		data.OpenTaskCount,
		data.InProgressTaskCount,
		data.OpenIncidentCount,
		data.CriticalIncidents,
	)
}

func chatbotSendMessage(c *gin.Context) {
	var body struct {
		OrganizationID int    `json:"organization_id"`
		ConversationID int64  `json:"conversation_id"`
		Message        string `json:"message"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	body.Message = strings.TrimSpace(body.Message)
	if body.Message == "" || len(body.Message) > 8000 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "message is required and must be at most 8000 characters",
		})
		return
	}

	organizationID, ok := resolveChatOrganization(c, body.OrganizationID)
	if !ok {
		return
	}

	userID := currentUserID(c)
	ctx := c.Request.Context()

	var conversationID int64

	if body.ConversationID > 0 {
		err := db.QueryRowContext(ctx, `
			SELECT id
			FROM chatbot_conversations
			WHERE id = $1 AND organization_id = $2 AND user_id = $3
		`, body.ConversationID, organizationID, userID).Scan(&conversationID)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
			return
		}
		if err != nil {
			fail(c, err)
			return
		}
	} else {
		title := body.Message
		if len(title) > 70 {
			title = title[:70]
		}

		err := db.QueryRowContext(ctx, `
			INSERT INTO chatbot_conversations
				(organization_id, user_id, title)
			VALUES ($1, $2, $3)
			RETURNING id
		`, organizationID, userID, title).Scan(&conversationID)

		if err != nil {
			fail(c, err)
			return
		}
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO chatbot_messages (conversation_id, role, content)
		VALUES ($1, 'user', $2)
	`, conversationID, body.Message)
	if err != nil {
		fail(c, err)
		return
	}

	data, err := loadChatbotContext(ctx, organizationID)
	if err != nil {
		fail(c, err)
		return
	}

	prompt := `You are OpsMind AI, an intelligent DevOps and SRE learning and workspace assistant.

You can answer BOTH:
1. Questions about the company's supplied workspace data.
2. General educational questions about DevOps, SRE, Linux, Docker, Kubernetes, CI/CD, cloud, networking, Git, scripting, monitoring, and troubleshooting.

IMPORTANT INSTRUCTIONS:
- Answer normal educational DevOps questions directly. Do not refuse harmless technical questions.
- You may provide practical examples, Dockerfiles, shell commands, configuration examples, and step-by-step explanations for legitimate learning and administration.
- For Docker image optimization, explain techniques such as small base images, multi-stage builds, .dockerignore, dependency caching, and removing unnecessary build dependencies. Include practical Dockerfile examples when requested.
- If the user requests Hindi and English, explain in simple Hindi/Hinglish and English.
- Match the detail level requested by the user. Use headings and code blocks when useful.
- Use the supplied workspace context for questions about this company's actual projects, tasks, incidents, logs, and metrics.
- Never invent workspace incidents, logs, metrics, project names, or system state.
- If workspace information is unavailable, clearly say that it is unavailable. Do not guess.
- Never claim to have executed commands, inspected live infrastructure, or changed infrastructure unless a connected tool actually performed that action.
- For practical troubleshooting, explain safe steps and distinguish example commands from actions actually performed.
- Refuse only requests that would meaningfully facilitate harm, such as malware deployment, credential theft, or unauthorized access. Do not treat ordinary Dockerfiles, DevOps commands, or infrastructure examples as harmful.
- Be accurate, clear, and useful. If uncertain, say so.

Company workspace context:
` + data.asPromptContext() + "\nUser question:\n" + body.Message

	reply, provider := "", "fallback"

	aiCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	if aiReply, aiErr := runOllama(aiCtx, prompt); aiErr == nil {
		reply = aiReply
		provider = "ollama"
	} else {
		reply = basicChatbotAnswer(body.Message, data)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO chatbot_messages (conversation_id, role, content)
		VALUES ($1, 'assistant', $2)
	`, conversationID, reply)
	if err != nil {
		fail(c, err)
		return
	}

	_, _ = db.ExecContext(ctx, `
		UPDATE chatbot_conversations
		SET updated_at = NOW()
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
	`, conversationID, organizationID, userID)

	c.JSON(http.StatusOK, gin.H{
		"conversation_id": conversationID,
		"reply":           reply,
		"provider":        provider,
	})
}

func chatbotListConversations(c *gin.Context) {
	requestedID, _ := strconv.Atoi(c.Query("organization_id"))

	organizationID, ok := resolveChatOrganization(c, requestedID)
	if !ok {
		return
	}

	rows, err := db.QueryContext(c.Request.Context(), `
		SELECT id, title, created_at, updated_at
		FROM chatbot_conversations
		WHERE organization_id = $1 AND user_id = $2
		ORDER BY updated_at DESC
		LIMIT 50
	`, organizationID, currentUserID(c))
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	type conversationItem struct {
		ID        int64     `json:"id"`
		Title     string    `json:"title"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	items := make([]conversationItem, 0)
	for rows.Next() {
		var item conversationItem
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			fail(c, err)
			return
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"conversations": items})
}

func chatbotGetConversation(c *gin.Context) {
	conversationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || conversationID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	requestedID, _ := strconv.Atoi(c.Query("organization_id"))
	organizationID, ok := resolveChatOrganization(c, requestedID)
	if !ok {
		return
	}

	var title string
	err = db.QueryRowContext(c.Request.Context(), `
		SELECT title
		FROM chatbot_conversations
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
	`, conversationID, organizationID, currentUserID(c)).Scan(&title)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}

	rows, err := db.QueryContext(c.Request.Context(), `
		SELECT id, role, content, created_at
		FROM chatbot_messages
		WHERE conversation_id = $1
		ORDER BY id ASC
	`, conversationID)
	if err != nil {
		fail(c, err)
		return
	}
	defer rows.Close()

	type messageItem struct {
		ID        int64     `json:"id"`
		Role      string    `json:"role"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}

	messages := make([]messageItem, 0)
	for rows.Next() {
		var item messageItem
		if err := rows.Scan(
			&item.ID,
			&item.Role,
			&item.Content,
			&item.CreatedAt,
		); err != nil {
			fail(c, err)
			return
		}
		messages = append(messages, item)
	}

	if err := rows.Err(); err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation_id": conversationID,
		"title":           title,
		"messages":        messages,
	})
}
