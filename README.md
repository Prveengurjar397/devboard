# OpsMind AI — AI-Assisted DevSecOps Incident Management Platform

**Detect. Investigate. Track. Resolve.**

OpsMind AI is a DevSecOps incident management platform designed to help development and operations teams track infrastructure incidents, investigate possible root causes, and coordinate remediation through automatically linked tasks.

The platform connects incident management with task tracking using a Go backend, React frontend, and PostgreSQL database.

> **Project status:** Hackathon MVP — actively under development.

---

## 1. Problem Statement

Infrastructure and application failures can create operational challenges for development and DevOps teams.

Common problems include:

- Incidents being tracked separately from development tasks.
- Manual effort required to create investigation tasks.
- Poor visibility into incident progress.
- Difficulty maintaining an incident activity history.
- Lack of synchronization between incident resolution and task completion.

OpsMind AI aims to bring incident tracking and task management together in one platform.

## 2. Project Overview

The current MVP provides:

- Centralized incident management.
- Incident activity timeline.
- Automatic task creation when an incident is created.
- Database-level relationship between incidents and tasks.
- Linked task information on the incident detail page.
- Task-to-incident status synchronization.

The architecture is designed to support future AI-assisted root-cause analysis and operational recommendations.

## 3. Architecture

```text
                  OpsMind AI
                       |
                React Frontend
                       |
                 API Gateway
                       |
                  Go Backend
                       |
              PostgreSQL Database
                       |
        +--------------+--------------+
        |              |              |
     Incidents       Tasks      Incident Events
        |              |              |
        +------ Linked Records -------+
                       |
              Status Synchronization
```

### Technology Stack

| Layer | Technologies |
|---|---|
| Frontend | React, Vite, JavaScript |
| Backend | Go, Gin |
| Database | PostgreSQL 16 |
| Containerization | Docker, Docker Compose |
| API | REST API |
| Monitoring integration | Python, Docker SDK |
| Local AI experimentation | Ollama, Llama 3.2 1B |
| Version control | Git, GitHub |

## 4. Implemented Features

### 4.1 Incident Management

- Create and view infrastructure incidents.
- Store incident title, description and container information.
- Track severity: Low, Medium, High and Critical.
- Track status: Open, Acknowledged, In Progress and Resolved.
- Store root cause, suggested fix and associated logs.

### 4.2 Automatic Task Creation

When a new incident is created:

1. The backend stores the incident in PostgreSQL.
2. An investigation task is automatically created.
3. The task is associated with the incident through a database relationship.
4. The task appears in the task management system.

### 4.3 Incident and Task Linking

- Each incident can have an associated investigation task.
- The relationship is persisted in PostgreSQL.
- Incident details display the linked task ID, title and status.
- Users can navigate to the task board.

### 4.4 Task-to-Incident Status Synchronization

When a linked task changes status, the backend can update the corresponding incident status.

Current implemented mappings:

| Task Status | Incident Status |
|---|---|
| Todo | Open |
| In Progress | In Progress |
| Blocked | Acknowledged |

A completed-task-to-resolved mapping is a planned behavior and should be verified before being considered complete.

Status changes are recorded in the incident activity timeline when synchronization occurs.

### 4.5 Incident Activity Timeline

The platform maintains incident events such as:

- Incident creation.
- Automatic task creation.
- Task status synchronization.

This provides a record of important incident-related activities.

### 4.6 Docker Monitoring Integration

The repository includes a Python-based Docker monitoring component intended to observe container events and send incident information to the backend.

The monitoring integration is an active development area; its complete end-to-end behavior should be validated before claiming production-ready monitoring.

---

## 5. Project Structure

```text
devboard/
│
├── backend/
│   ├── main.go
│   ├── incidents.go
│   ├── incident_events.go
│   └── Dockerfile
│
├── frontend/
│   └── src/
│       ├── api/
│       ├── components/
│       ├── hooks/
│       └── pages/
│
├── migrations/
│   ├── 001_incident_events.sql
│   └── 002_incident_task_link.sql
│
├── monitoring/
│   └── docker_monitor.py
│
├── init/
│   └── postgres/
│
├── docker-compose.yml
├── Makefile
├── .env.example
└── README.md
```

---

## 6. Getting Started

### Prerequisites

- Docker Desktop or Docker Engine.
- Docker Compose.
- Git.

Go, Node.js and PostgreSQL do not need to be installed locally when using the Docker-based setup.

### Step 1: Clone the Repository

```bash
git clone https://github.com/Prveengurjar397/devboard.git
```

### Step 2: Enter the Project

```bash
cd devboard
```

### Step 3: Switch to the OpsMind AI Branch

```bash
git checkout opsmind-ai
```

### Step 4: Configure Environment

```bash
cp .env.example .env
```

Review `.env.example` and configure the required local settings in `.env`.

Do not commit your actual `.env` file or expose credentials.

### Step 5: Build and Start the Application

```bash
docker compose up --build -d
```

### Step 6: Verify the Services

```bash
docker compose ps
```

Check the backend health endpoint:

```bash
curl http://localhost:8081/health
```

Check the application API through the frontend gateway:

```bash
curl "http://localhost:8080/api/tasks?project_id=1"
```

### Step 7: Open the Application

Frontend:

http://localhost:8080

Backend health:

http://localhost:8081/health

### Stop the Application

```bash
docker compose down
```

This stops the containers without intentionally deleting the PostgreSQL data volume.

---

## 7. API Endpoints

The following endpoints are available in the current backend.

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/health` | Backend health check |
| GET | `/projects` | List projects |
| POST | `/projects` | Create project |
| GET | `/tasks` | List tasks |
| POST | `/tasks` | Create task |
| PATCH | `/tasks/:id` | Update task |
| GET | `/search` | Search tasks |
| GET | `/incidents` | List incidents |
| POST | `/incidents` | Create incident |
| GET | `/incidents/:id` | Get incident details |
| GET | `/incidents/:id/events` | Get incident timeline |

Frontend API requests use the `/api` prefix, which is forwarded to the backend.

---

## 8. Database Design

The platform uses PostgreSQL to persist project, task and incident information.

Main tables:

| Table | Responsibility |
|---|---|
| `projects` | Project information |
| `tasks` | Task management |
| `incidents` | Infrastructure incidents |
| `incident_events` | Incident activity history |

### Incident-to-Task Relationship

The `tasks` table contains an `incident_id` foreign key referencing `incidents.id`.

A unique partial index ensures that an incident is associated with at most one linked task.

Database migrations are maintained in the `migrations/` directory.

For an existing database, apply the migrations in numerical order using the migration instructions appropriate to your database setup.

---

## 9. Current Limitations

OpsMind AI is a hackathon MVP, not a production-ready incident management system.

The following capabilities are not yet fully implemented:

- User authentication and registration.
- User-wise access control and data isolation.
- AI chatbot interface.
- Fully validated AI-powered root-cause analysis workflow.
- Verified task completion-to-incident resolution synchronization.
- Production-grade monitoring, alerting and security controls.

The platform should not be used as a production incident response system in its current state.

---

## 10. Future Roadmap

Planned development areas:

### Phase 1 — Core Incident Management
- [x] Incident creation and management
- [x] Incident activity timeline
- [x] Automatic investigation task creation
- [x] Incident-task database relationship
- [x] Linked task visibility in incident details
- [x] Task-to-incident status synchronization (implemented mappings)

### Phase 2 — Authentication and Access Control
- [ ] User registration and login
- [ ] Password hashing
- [ ] Protected backend APIs
- [ ] User-wise data ownership

### Phase 3 — AI Assistance
- [ ] AI chatbot interface
- [ ] AI-assisted log analysis
- [ ] Root-cause analysis recommendations
- [ ] Suggested remediation steps
- [ ] Incident investigation assistance

### Phase 4 — DevSecOps and Observability
- [ ] Enhanced Docker monitoring
- [ ] Monitoring dashboards
- [ ] Security scanning integration
- [ ] CI/CD pipeline improvements
- [ ] Incident metrics and reporting

**Note:** Roadmap items are planned capabilities, not claims of existing functionality.

---

## 11. Project Goals

OpsMind AI aims to make incident investigation and task coordination more organized by connecting infrastructure events, incident records, and development workflows.

The long-term vision is to build an AI-assisted operational platform that helps teams understand incidents and make informed remediation decisions, with human approval for recovery actions.

---

## 12. Repository

**GitHub:** https://github.com/Prveengurjar397/devboard

**Development Branch:** `opsmind-ai`

---

**OpsMind AI — Bringing Incident Management, Task Automation and AI-Assisted Operations Together.**