---
title: API Reference
description: Explore the REST API of Leap
---

The Leap backend exposes a REST API. The API uses JSON for requests and responses. The frontend consumes this API.

## Base URL

The API lives under `/api`. In development, the frontend proxy forwards `/api/*` to the backend at `localhost:9000`.

## Authentication

The API uses JWT tokens.

- `POST /api/auth/login` returns an access token and a refresh cookie.
- The access token is short-lived. It is kept in memory.
- The refresh token lives in an HttpOnly cookie.
- `POST /api/auth/refresh` issues a new access token.
- `POST /api/auth/logout` invalidates the refresh token.

Cookie-authenticated requests need a CSRF token. Login and refresh endpoints are rate-limited.

## Response format

A success response has this shape:

```json
{ "data": { ... }, "error": null, "meta": { "page": 1, "per_page": 20, "total": 150 } }
```

An error response has this shape:

```json
{ "data": null, "error": { "code": "FORBIDDEN", "message": "Insufficient permissions" } }
```

The `meta` object carries pagination information.

## Endpoint groups

### Auth

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/auth/login` | Log in with email and password |
| `POST` | `/api/auth/refresh` | Get a new access token |
| `POST` | `/api/auth/logout` | Log out |
| `GET` | `/api/auth/me` | Get the current user |
| `PATCH` | `/api/auth/me` | Update the profile |
| `PATCH` | `/api/auth/me/password` | Change the password |
| `GET` | `/api/auth/me/permissions` | Get resolved permissions |

### Contacts

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/contacts` | List contacts |
| `POST` | `/api/contacts` | Create a contact |
| `POST` | `/api/contacts/bulk` | Import contacts from CSV |
| `GET` | `/api/contacts/resolve` | Resolve a phone for lead entry |
| `GET` | `/api/contacts/:id` | Get a contact |
| `PATCH` | `/api/contacts/:id` | Update a contact |
| `DELETE` | `/api/contacts/:id` | Delete a contact |
| `GET` | `/api/contacts/:id/notes` | List notes |
| `POST` | `/api/contacts/:id/notes` | Add a note |
| `DELETE` | `/api/contacts/:id/notes/:note_id` | Delete a note |

### Leads

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/leads` | List leads |
| `GET` | `/api/leads/board` | Get the kanban board |
| `POST` | `/api/leads` | Create a lead |
| `GET` | `/api/leads/:id` | Get a lead |
| `PATCH` | `/api/leads/:id` | Update a lead |
| `DELETE` | `/api/leads/:id` | Delete a lead |
| `GET` | `/api/leads/:id/activities` | List activities |
| `POST` | `/api/leads/:id/activities` | Add an activity |
| `PATCH` | `/api/leads/:id/activities/:id` | Update an activity |
| `DELETE` | `/api/leads/:id/activities/:id` | Delete an activity |
| `GET` | `/api/leads/:id/history` | Get stage-move history |

### Tasks and reminders

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/reminders` | List pending reminders |
| `GET` | `/api/activities` | List all tasks |
| `PATCH` | `/api/leads/:lead_id/reminders/:id` | Dismiss a reminder |
| `POST` | `/api/leads/:lead_id/reminders/:id/snooze` | Snooze a reminder |

### Settings

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/pipelines` | List pipelines |
| `POST` | `/api/pipelines` | Create a pipeline |
| `PATCH` | `/api/pipelines/:id` | Update a pipeline |
| `DELETE` | `/api/pipelines/:id` | Delete a pipeline |
| `POST` | `/api/pipelines/:id/stages` | Add a stage |
| `PUT` | `/api/pipelines/:id/stages/order` | Reorder the stages |
| `PATCH` | `/api/stages/:stage_id` | Update a stage |
| `DELETE` | `/api/stages/:stage_id` | Delete a stage |
| `GET` | `/api/programs` | List active programs |
| `GET` | `/api/programs/manage` | List all programs, including archived |
| `POST` | `/api/programs` | Create a program |
| `PATCH` | `/api/programs/:id` | Update a program |
| `DELETE` | `/api/programs/:id` | Archive a program |
| `POST` | `/api/programs/:id/restore` | Restore a program |
| `GET` | `/api/tags` | List tags |
| `POST` | `/api/tags` | Create a tag |
| `PATCH` | `/api/tags/:id` | Update a tag |
| `DELETE` | `/api/tags/:id` | Delete a tag |
| `GET` | `/api/roles` | List roles |
| `POST` | `/api/roles` | Create a role |
| `PATCH` | `/api/roles/:id` | Update a role |
| `DELETE` | `/api/roles/:id` | Delete a role |
| `GET` | `/api/roles/:id/permissions` | List role permissions |
| `POST` | `/api/roles/:id/permissions` | Add a role permission |
| `PUT` | `/api/roles/:id/permissions` | Set the role permissions |
| `DELETE` | `/api/roles/:id/permissions/:id` | Remove a role permission |
| `GET` | `/api/permissions` | List all permissions |
| `GET` | `/api/users` | List users |
| `GET` | `/api/users/options` | List assignee options |
| `POST` | `/api/users` | Create a user |
| `PATCH` | `/api/users/:id` | Update a user |
| `DELETE` | `/api/users/:id` | Delete a user |
| `POST` | `/api/users/:id/reset-password` | Reset a password |
| `POST` | `/api/users/:id/reactivate` | Reactivate a user |
| `PUT` | `/api/users/:id/role` | Set the user role |

### Organization settings

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/settings/nudge-lead-minutes` | Get the reminder lead time |
| `PUT` | `/api/settings/nudge-lead-minutes` | Set the reminder lead time |
| `GET` | `/api/settings/default-country-code` | Get the default country code |
| `PUT` | `/api/settings/default-country-code` | Set the default country code |

### Audit and export

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/activity` | Read the audit log |
| `GET` | `/api/export/csv` | Export data as CSV |

### Health

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Check that the process is alive |
| `GET` | `/readyz` | Check that the database is ready |

## Permissions

Most routes require a permission. The middleware checks the permission before the handler runs, and a request without it returns `FORBIDDEN`. The org-setting reads and the self-service auth routes are open to every signed-in user. The [Features](/features/) page lists every permission.
