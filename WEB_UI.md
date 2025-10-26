# Web UI Documentation

## Overview

The GraphDB Service includes a modern web interface built with **Templ** and **HTMX** for executing and monitoring GraphDB migration tasks in real-time.

## Features

- ✅ **JSON Task Upload**: Paste or load example JSON task definitions
- ✅ **Real-Time Progress**: Watch task execution with live status updates via Server-Sent Events
- ✅ **Task States**: Visual indicators for pending, in-progress, success, error, and timeout states
- ✅ **Server-Sent Events (SSE)**: Real-time updates without polling
- ✅ **Session Management**: Each execution gets a unique session ID
- ✅ **Responsive Design**: Clean, modern UI with color-coded status indicators
- ✅ **No Authentication Required**: UI is publicly accessible (API still requires API key)
- ✅ **Auto-Cleanup**: Sessions automatically cleaned up after 1 hour

## Technology Stack

- **Templ**: Type-safe Go templating for HTML generation
- **HTMX**: Dynamic HTML updates without JavaScript frameworks
- **Server-Sent Events (SSE)**: Real-time task status streaming
- **Echo**: Web framework for routing and middleware

## Accessing the Web UI

### Start the Service

```bash
# Start the service (API key not required for UI)
go run main.go graphdb

# Or use the compiled binary
./graphservice graphdb

# Optional: Set API key for programmatic API access
export API_KEY=your-secret-key
```

### Open in Browser

Navigate to:
- **http://localhost:8080/** (main page)
- **http://localhost:8080/ui** (alternative route)

## Using the Web UI

### 1. Submit Tasks

1. **Paste JSON**: Copy your task definition into the textarea
2. **Load Example**: Click "Load example" to see a sample JSON structure
3. **Execute**: Click "Execute Tasks" button to start

### 2. Monitor Execution

Once submitted, you'll see a list of all tasks with:

- **Task Number**: Sequential task identifier (Task 1, Task 2, etc.)
- **Action Type**: The operation being performed (e.g., repo-migration, graph-migration)
- **Status Badge**: Color-coded status indicator
- **Source/Target**: Repository and graph information
- **Progress Messages**: Real-time execution details

### 3. Task States

| State | Color | Description |
|-------|-------|-------------|
| **Pending** | Gray | Task waiting to start |
| **In Progress** | Blue | Task currently executing (with spinner animation) |
| **Success** | Green | Task completed successfully |
| **Error** | Red | Task failed with an error |
| **Timeout** | Yellow | Task exceeded 10-minute timeout |

## Example JSON

### Repository Migration

```json
{
  "version": "v0.0.1",
  "tasks": [
    {
      "action": "repo-migration",
      "src": {
        "url": "http://source-graphdb:7200",
        "username": "admin",
        "password": "password",
        "repo": "source-repo"
      },
      "tgt": {
        "url": "http://target-graphdb:7200",
        "username": "admin",
        "password": "password",
        "repo": "target-repo"
      }
    }
  ]
}
```

### Graph Migration

```json
{
  "version": "v0.0.1",
  "tasks": [
    {
      "action": "graph-migration",
      "src": {
        "url": "http://source-graphdb:7200",
        "username": "admin",
        "password": "password",
        "repo": "source-repo",
        "graph": "http://example.org/my-graph"
      },
      "tgt": {
        "url": "http://target-graphdb:7200",
        "username": "admin",
        "password": "password",
        "repo": "target-repo",
        "graph": "http://example.org/my-graph"
      }
    }
  ]
}
```

### Multiple Tasks

```json
{
  "version": "v0.0.1",
  "tasks": [
    {
      "action": "repo-delete",
      "tgt": {
        "url": "http://graphdb:7200",
        "username": "admin",
        "password": "password",
        "repo": "old-repo"
      }
    },
    {
      "action": "repo-migration",
      "src": {
        "url": "http://source:7200",
        "username": "admin",
        "password": "password",
        "repo": "source-repo"
      },
      "tgt": {
        "url": "http://target:7200",
        "username": "admin",
        "password": "password",
        "repo": "new-repo"
      }
    }
  ]
}
```

## Architecture

### Components

#### 1. Templ Templates (`web/templates/`)

- **layout.templ**: Base HTML layout with CSS and HTMX scripts
  - Responsive design with CSS Grid and Flexbox
  - Modern color palette with CSS variables
  - Animation for task state transitions

- **index.templ**: Main page with JSON upload form
  - HTMX-powered form submission
  - Example JSON loader
  - Client-side validation

- **tasks.templ**: Task list and real-time status components
  - TaskResults: Container with SSE connection
  - TaskItem: Individual task display
  - TaskUpdate: SSE update component

#### 2. Web Handlers (`cmd/web_handlers.go`)

- **uiIndexHandler**: Serves the main UI page
- **uiExecuteHandler**: Processes task submissions
  - Validates JSON format
  - Creates execution session
  - Starts background task processing
  - Returns initial task list

- **uiStreamHandler**: Manages SSE connections for real-time updates
  - Sets proper SSE headers
  - Registers client connections
  - Streams task updates as they occur
  - Handles client disconnection

- **executeTasksWithUpdates**: Background task execution with status broadcasting
  - Validates each task
  - Executes tasks sequentially
  - Broadcasts status updates via SSE
  - Handles errors and timeouts

#### 3. Session Management

Each task execution creates a session with:
- Unique UUID identifier
- Task status array
- Map of connected SSE clients
- Start/end timestamps
- Thread-safe mutex for concurrent access
- Automatic cleanup after 1 hour

### Data Flow

```
User submits JSON
    ↓
uiExecuteHandler validates and creates session
    ↓
Returns initial task list HTML with session ID
    ↓
Browser connects to SSE stream (/ui/stream/{sessionID})
    ↓
executeTasksWithUpdates runs in background goroutine
    ↓
Each task status change is broadcast to all connected clients
    ↓
HTMX receives SSE event and swaps updated task HTML
    ↓
UI reflects current state in real-time
```

## Routes

| Method | Path | Description | Auth Required |
|--------|------|-------------|---------------|
| GET | `/` | Main UI page | No |
| GET | `/ui` | Main UI page (alternative) | No |
| POST | `/ui/execute` | Submit tasks for execution | No |
| GET | `/ui/stream/:sessionID` | SSE stream for task updates | No |
| POST | `/v1/api/action` | API endpoint (JSON/multipart) | Yes (API key) |
| GET | `/health` | Health check | No |

## Real-Time Updates

### Server-Sent Events (SSE)

The UI uses SSE for real-time updates via HTMX's SSE extension:

```html
<div hx-ext="sse"
     sse-connect="/ui/stream/session-uuid"
     sse-swap="task-update">
```

**Benefits:**
- One-way server-to-client communication
- Automatic reconnection on connection loss
- Event-based message delivery
- Works through firewalls (HTTP)
- Lower overhead than WebSockets for this use case

### Task Update Events

Each task status change sends an SSE event:

```
event: task-update
data: <li class="task-item success">...</li>
```

HTMX receives the event and updates the corresponding task item in the DOM using `hx-swap="none"` with custom JavaScript.

## Configuration

### Environment Variables

```bash
# HTTP server port (default: 8080)
export PORT=8080

# API key for /v1/api/action endpoint (not required for UI)
export API_KEY=your-secret-key
```

### Customization

#### Timeout Duration

Modify in `cmd/web_handlers.go`:

```go
case <-time.After(10 * time.Minute): // Change timeout here
```

#### Session Cleanup

Modify in `cmd/web_handlers.go`:

```go
time.AfterFunc(1*time.Hour, func() { // Change cleanup duration here
    sessionsMutex.Lock()
    delete(sessions, sessionID)
    sessionsMutex.Unlock()
})
```

#### Styling

Edit CSS in `web/templates/layout.templ`:

```css
:root {
    --primary: #2563eb;    /* Change primary color */
    --success: #10b981;    /* Change success color */
    --error: #ef4444;      /* Change error color */
    /* ... */
}
```

## Development

### Regenerate Templ Templates

After modifying `.templ` files:

```bash
# Using task command
task templ:generate

# Or directly
templ generate
```

This generates corresponding `_templ.go` files in the same directory.

### Project Structure

```
pxgraphservice/
├── cmd/
│   ├── graphdb.go          # Main server with routes
│   ├── web_handlers.go     # UI request handlers
│   └── graphdb_test.go     # Tests
├── web/
│   └── templates/
│       ├── layout.templ    # Base layout
│       ├── index.templ     # Main page
│       ├── tasks.templ     # Task components
│       ├── layout_templ.go # Generated
│       ├── index_templ.go  # Generated
│       └── tasks_templ.go  # Generated
├── taskfile.yml            # Task automation
└── WEB_UI.md              # This file
```

### Build Workflow

```bash
# 1. Modify Templ templates
vim web/templates/layout.templ

# 2. Generate Go code
task templ:generate

# 3. Build application
go build .

# 4. Run
./graphservice graphdb
```

## Security Considerations

### API vs UI Authentication

- **UI Routes (`/`, `/ui/*`)**: No authentication required
  - Users can submit and monitor tasks
  - Suitable for internal networks
  - Consider adding auth for production

- **API Routes (`/v1/api/action`)**: API key required
  - Programmatic access requires `x-api-key` header
  - Prevents unauthorized automated access

### Production Recommendations

1. **Use HTTPS**: Always run behind a reverse proxy with TLS
2. **Add Authentication**: Implement OAuth2, JWT, or basic auth for UI
3. **Rate Limiting**: Prevent abuse with rate limiting middleware
4. **Input Validation**: UI validates JSON, but add server-side limits
5. **CORS Configuration**: Restrict allowed origins in production
6. **Session Limits**: Limit number of active sessions per user
7. **Content Security Policy**: Add CSP headers to prevent XSS

### Adding Basic Authentication

To add basic auth to UI routes:

```go
// In cmd/graphdb.go
uiAuth := middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
    return username == "admin" && password == os.Getenv("UI_PASSWORD"), nil
})

// Apply to UI routes
e.GET("/", uiIndexHandler, uiAuth)
e.GET("/ui", uiIndexHandler, uiAuth)
e.POST("/ui/execute", uiExecuteHandler, uiAuth)
e.GET("/ui/stream/:sessionID", uiStreamHandler, uiAuth)
```

## Troubleshooting

### UI Not Loading

**Problem**: Blank page or 404 errors

**Solution**:
```bash
# Ensure templ files are generated
task templ:generate

# Verify generated files exist
ls web/templates/*_templ.go

# Rebuild the application
go build .

# Check for compilation errors
go build -v .
```

### SSE Not Working

**Problem**: Task status not updating in real-time

**Solution**:
- Check browser console for SSE connection errors
- Verify session ID is correct in Network tab
- Ensure no proxy is buffering SSE responses
- Check server logs for connection issues
- Test with curl: `curl -N http://localhost:8080/ui/stream/{sessionID}`

### Tasks Timeout

**Problem**: All tasks show timeout status

**Solution**:
- Verify GraphDB instances are accessible
- Check network connectivity to GraphDB servers
- Increase timeout duration in code (default 10 minutes)
- Review server logs for specific errors
- Test GraphDB connectivity separately

### High Memory Usage

**Problem**: Memory increases with many sessions

**Solution**:
- Sessions auto-cleanup after 1 hour
- Reduce cleanup duration for high-traffic scenarios
- Monitor active sessions count
- Consider implementing max concurrent sessions limit
- Use `pprof` to profile memory usage

### CSS Not Applying

**Problem**: Styles missing or incorrect

**Solution**:
- Styles are embedded in layout.templ
- No external CSS files needed
- Check browser developer tools for CSS errors
- Verify layout.templ was regenerated after changes
- Clear browser cache

## Browser Compatibility

### Fully Supported
- ✅ Chrome/Edge 90+
- ✅ Firefox 88+
- ✅ Safari 14+
- ✅ Opera 76+

### Requirements
- Server-Sent Events (SSE) support
- Modern CSS (CSS Grid, Flexbox, CSS Variables)
- JavaScript enabled (for HTMX and example loader)

## Performance

### Scalability

- **Concurrent Sessions**: Limited only by memory
- **SSE Connections**: Each session can have multiple browser connections
- **Task Throughput**: Sequential execution per session (by design)
- **Memory Usage**: ~100KB per active session

### Optimization Tips

1. **Session Cleanup**: Adjust cleanup interval based on usage patterns
2. **SSE Buffer**: Client channels have 10-message buffer
3. **Goroutines**: One goroutine per task execution
4. **Connection Pooling**: Reuses HTTP connections to GraphDB

## Future Enhancements

### Planned Features
- [ ] Task history view with search
- [ ] Export execution results (JSON, CSV)
- [ ] Pause/resume task execution
- [ ] Task scheduling and queuing
- [ ] Multi-user sessions with isolation
- [ ] WebSocket support (alternative to SSE)
- [ ] Task cancellation
- [ ] Detailed logs viewer with filtering
- [ ] GraphDB connection testing before execution
- [ ] Dark mode toggle
- [ ] Email notifications on completion
- [ ] Metrics dashboard (success rate, avg duration)

### Contributing

To add new features:

1. **Add Templ components** in `web/templates/*.templ`
2. **Generate code**: `task templ:generate`
3. **Add handlers** in `cmd/web_handlers.go`
4. **Register routes** in `cmd/graphdb.go`
5. **Test thoroughly** with different task types
6. **Update documentation** in this file

## References

- **Templ**: https://templ.guide/
- **HTMX**: https://htmx.org/
- **HTMX SSE Extension**: https://htmx.org/extensions/server-sent-events/
- **Server-Sent Events**: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events
- **Echo Framework**: https://echo.labstack.com/

## Example Session Flow

```
1. User loads http://localhost:8080/
   → uiIndexHandler renders Index template

2. User pastes JSON and clicks "Execute Tasks"
   → HTMX sends POST to /ui/execute
   → uiExecuteHandler:
      - Validates JSON
      - Creates session with UUID
      - Initializes task statuses
      - Starts executeTasksWithUpdates goroutine
      - Returns TaskResults template with session ID

3. Browser receives HTML with session ID
   → HTMX SSE extension connects to /ui/stream/{sessionID}
   → uiStreamHandler:
      - Registers client channel
      - Sends current task states
      - Keeps connection open

4. Background goroutine executes tasks
   → For each task:
      - Updates status to "in-progress"
      - Broadcasts update to all clients
      - Executes task (calls processTask)
      - Updates status to "success"/"error"/"timeout"
      - Broadcasts final update

5. Browser receives SSE events
   → HTMX swaps task HTML on each update
   → UI shows real-time progress

6. After 1 hour
   → Session automatically cleaned up
   → Memory released
```

---

**Last Updated**: 2025-10-26
**Version**: v0.0.3
**License**: Apache 2.0
