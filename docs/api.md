# Multiplayer Asset — API Documentation

This document describes the public HTTP API and the WebSocket matchmaking
endpoint exposed by the `api` service. All endpoints are served over the same
origin as the web frontend.

- Base path for REST endpoints: `/api`
- Content type for request bodies and responses: `application/json` (unless
  stated otherwise)
- WebSocket endpoint: `/api/connection`

## Authentication

Most endpoints require an authenticated session. Sessions are established by
`POST /api/login` or `POST /api/register`, which set a cookie named `session`.

When the `session` cookie is absent, endpoints respond with HTTP `401`:

```json
{ "msg": "No session token provided" }
```

When the `session` cookie is present but its value is not a valid token (or the
token does not correspond to a live session), endpoints respond with HTTP `401`:

```json
{ "msg": "Invalid session token" }
```

## POST `/api/register`

Creates a new user account and immediately establishes a session for that user.

**Request body**

```json
{
    "login": "string",
    "password": "string"
}
```

**Responses**

| Code | Response                 | Description                                              |
| ------ | -------------------- | -------------------------------------------------------- |
| 200    | `{}`                 | Account created and session established; the session cookie is set automatically. |
| 400    | `{ "msg": string }`  | Request body is not valid JSON.                          |
| 422    | `{ "msg": string }`  | The request body is a valid JSON object but the credentials are missing or malformed (e.g. wrong type or length). |
| 409    | `{ "msg": "Login is already taken" }` | A user with the given `login` already exists. |

The `login` must be 3–20 characters and `password` must be 5–64 characters.

---

## POST `/api/login`

Authenticates a user and establishes a session.

**Request body**

```json
{
    "login": "string",
    "password": "string"
}
```

**Responses**

| Code | Response                 | Description                                              |
| ------ | -------------------- | -------------------------------------------------------- |
| 200    | `{}`                 | Session created. The session cookie is set automatically and the token is persisted server-side. |
| 400    | `{ "msg": string }`  | Request body is not valid JSON.                          |
| 422    | `{ "msg": string }`  | The request body is a valid JSON object but the credentials are missing or malformed (e.g. wrong type or length). |
| 401    | `{ "msg": "Invalid login or password" }` | Credentials are incorrect, or the account no longer exists. |

The `login` and `password` length limits are the same as for registration.

---

## POST `/api/logout`

Terminates the current session and clears the session cookie.

Requires the `session` cookie. If the cookie is absent or invalid, the session
is not terminated.

**Request body**

None.

**Responses**

| Code | Response | Description                             |
| ------ | ---- | --------------------------------------- |
| 200    | `{}` | Session deleted; the session cookie is cleared automatically. |
| 401    | `{ "msg": "No session token provided" }` | No `session` cookie present. |
| 401    | `{ "msg": "Invalid session token" }` | Cookie value is not a valid token. |

Note: the session cookie is cleared on success regardless of whether the
referenced session still exists.

---

## GET `/api/session`

Reports the login of the session identified by the `session` cookie. When the
cookie is well-formed but the session no longer exists, the cookie is deleted.

**Request body**

None.

**Responses**

| Code | Response                       | Description                              |
| ------ | -------------------------- | ---------------------------------------- |
| 200    | `{ "login": string }`     | The login the session belongs to. |
| 200    | `{ "login": null }`       | The session no longer exists; the stale cookie is deleted. |
| 401    | `{ "msg": "No session token provided" }` | No `session` cookie present. |
| 401    | `{ "msg": "Invalid session token" }`     | Cookie value is not a valid token. |

---

## GET `/api/match`

Returns the active match assigned to the authenticated user, if any.

**Request body**

None.

**Responses**

| Code | Response                              | Description                                              |
| ------ | ------------------------------------- | -------------------------------------------------------- |
| 200    | `{ "match": { "host": "string", "port": "string", "matchAuthToken": "string" } }` | The user is currently assigned to a match. |
| 200    | `{ "match": null }`                   | The user has no active match.                            |
| 401    | `{ "msg": "No session token provided" }` | No `session` cookie present.                           |
| 401    | `{ "msg": "Invalid session token" }`  | Cookie value is invalid or the session no longer exists. |

`host` and `port` are the connection details of the user-provided game server
assigned to the match. `matchAuthToken` is the token the client must present to
that game server to join the match.

---

## GET `/api/results`

Returns the historical match results for the authenticated user.

**Request body**

None.

**Responses**

| Code | Response                                                                                                                                                   | Description                                                   |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------- |
| 200    | `{ "matches": [ { "details": <object>, "canceled": boolean } ] }`                                                                                      | List of past matches for the user.       |
| 401    | `{ "msg": "No session token provided" }`                                                                                                               | No `session` cookie present.                                  |
| 401    | `{ "msg": "Invalid session token" }`                                                                                                                   | Cookie value is invalid or the session no longer exists.      |

`details` is an opaque object supplied by the game server that describes the
outcome of the match (for example `{ "players": [...], "winner": "..." }`).
`canceled` is `true` when the match was canceled rather than played to
completion.

---

## GET `/api/connection`

The matchmaking WebSocket endpoint cannot be upgraded over plain HTTP. A GET
request against this path is deliberately rejected. The actual WebSocket
handshake is handled by a dedicated server outside of SvelteKit.

**Request body**

None.

**Responses**

| Code | Response                          | Description                                    |
| ------ | --------------------------------- | ---------------------------------------------- |
| 426    | `{ "msg": "Websocket is required" }` | A WebSocket upgrade is required.              |

---

# WebSocket `/api/connection`

The matchmaking WebSocket. Clients connect to open a matchmaking queue entry;
the server responds with match details once a match is assigned, then closes the
connection.

## Establishing a Connection

- **Protocol:** WebSocket (`ws://` or `wss://` depending on deployment).
- **Path:** `/api/connection`.
- The handshake must be a valid WebSocket upgrade (e.g. `Sec-WebSocket-Key`,
  `Sec-WebSocket-Version: 13`). Requests to other paths are rejected.

### Authentication

Authentication is performed via the `session` cookie carried on the handshake
request. The cookie value must be a valid hexadecimal session token that maps to
an active session.

The handshake is validated **before** the connection is accepted. Connections
that fail authentication are closed immediately with a close code (see
[Close codes](#close-codes-and-reasons)).

### Application Protocol

After the handshake the server does **not** require any application-level frames
to be sent by the client. Clients may remain passive. All communication is
driven by the server.

## What the Server Sends

### Ping frames

The server sends a WebSocket **ping** control frame periodically. The ping
payload contains a numeric sequence (`"0"`, `"1"`, `"2"`, ...). The client is
expected to respond with a matching **pong** frame carrying the same sequence
payload.

A pong with a wrong sequence is ignored. If the client misses the pong deadline,
the miss is counted; after too many consecutive missed pongs the server
terminates the connection (see `1006` in [Close codes](#close-codes-and-reasons)).

### Match assignment (text frame)

Once a match is assigned to the queued user, the server sends a single JSON text
frame and then closes the connection with code `1000`:

```json
{
    "login": "string",
    "host": "string",
    "port": "string",
    "matchAuthToken": "string"
}
```

| Field           | Type   | Description                                                    |
| --------------- | ------ | -------------------------------------------------------------- |
| `login`         | string | The authenticated user's login.                                |
| `host`          | string | Address of the user-provided game server assigned to the match.|
| `port`          | string | Port of the game server.                                       |
| `matchAuthToken`| string | Token the client must present to the game server to join.      |

After sending this frame the server closes the connection normally. Clients
should treat receipt of this frame as a successful matchmaking result.

### No other messages

No other application-level messages are emitted. A connection that is still
queued simply remains open, receiving periodic pings, until a match is assigned
or the connection is closed for one of the reasons below.

## What the Client Must Do

1. Connect to `/api/connection` with the `session` cookie set.
2. Respond to every server **ping** with a **pong** carrying the same payload
   (and do so within the pong deadline, repeatedly), otherwise the connection is
   terminated.
3. Wait for the match-assignment text frame; once received, the connection will
   be closed by the server.

The client may close the connection at any time (e.g. to leave the queue).

## Close Codes and Reasons

The server may close the connection with the following codes. Application-level
reasons are sent in the close reason string.

### 1000 — Normal closure
- **Reason:** `""` (empty)
- **Description:** A match was assigned and the details were delivered. This is
  the expected successful outcome.

### 1001 — Server shutting down
- **Reasons:**
  - `Server is shutting down`
- **Description:** The server is shutting down or the connection server is not
  open. The connection cannot be served.

### 1006 — Abnormal closure
- **Reason:** No close frame is sent.
- **Description:** The client failed to respond to pings in time (too many
  consecutive missed pongs) and the server terminated the TCP connection, or the
  connection dropped abnormally.

### 1011 — Internal error
- **Reasons:**
  - `Internal error`
- **Description:** The server failed to register the queue entry or failed to
  authenticate the connection due to an unexpected error. The client may retry.

### 4000 — Already in match
- **Reason:** `Already in match`
- **Description:** The user already has a match assigned, so they cannot be
  queued. The connection is rejected immediately after the handshake.

### 4001 — Replaced by new connection
- **Reasons:**
  - `Replaced by new connection`
- **Description:** Another, newer WebSocket was opened for the same login. The
  existing connection is closed in favor of the newest one. Only the newest
  connection is tracked in the queue.

### 4004 — User no longer exists
- **Reason:** `User no longer exists`
- **Description:** The account associated with the session was removed while the
  connection was queued.

### 4401 — Authentication failed
- **Reasons:**
  - `No cookie`
  - `Cookie header too large`
  - `No session token`
  - `Invalid session token`
- **Description:** The handshake did not carry a valid `session` cookie. This
  code is sent during the upgrade handshake (before the connection is fully
  established) for missing, oversized, malformed, or expired tokens. The
  connection is not accepted.
