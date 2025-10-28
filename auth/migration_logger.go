package auth

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"
)

// MigrationSession represents a complete migration request
type MigrationSession struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	Username       string          `json:"username"`
	IPAddress      string          `json:"ip_address"`
	UserAgent      string          `json:"user_agent"`
	StartTime      time.Time       `json:"start_time"`
	EndTime        *time.Time      `json:"end_time,omitempty"`
	Duration       int64           `json:"duration_ms"`
	Status         string          `json:"status"` // running, completed, failed, timeout
	TotalTasks     int             `json:"total_tasks"`
	CompletedTasks int             `json:"completed_tasks"`
	FailedTasks    int             `json:"failed_tasks"`
	TimeoutTasks   int             `json:"timeout_tasks"`
	TotalDataSize  int64           `json:"total_data_size_bytes"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	Tasks          []MigrationTask `json:"tasks"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// MigrationTask represents a single task within a migration
type MigrationTask struct {
	TaskIndex   int        `json:"task_index"`
	Action      string     `json:"action"`
	SourceURL   string     `json:"source_url"`
	TargetURL   string     `json:"target_url"`
	RepoID      string     `json:"repo_id,omitempty"`
	GraphID     string     `json:"graph_id,omitempty"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Duration    int64      `json:"duration_ms"`
	Status      string     `json:"status"` // pending, running, success, error, timeout
	DataSize    int64      `json:"data_size_bytes"`
	TripleCount int64      `json:"triple_count,omitempty"`
	ErrorType   string     `json:"error_type,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	RetryCount  int        `json:"retry_count"`
	FileInfo    *FileInfo  `json:"file_info,omitempty"`
}

// FileInfo captures uploaded file metadata
type FileInfo struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size_bytes"`
	MD5Hash     string `json:"md5_hash,omitempty"`
}

// MigrationDailySummary contains aggregated statistics for a day
type MigrationDailySummary struct {
	Date             string              `json:"date"`
	TotalSessions    int                 `json:"total_sessions"`
	CompletedSessions int                `json:"completed_sessions"`
	FailedSessions   int                 `json:"failed_sessions"`
	RunningSessions  int                 `json:"running_sessions"`
	TotalTasks       int                 `json:"total_tasks"`
	CompletedTasks   int                 `json:"completed_tasks"`
	FailedTasks      int                 `json:"failed_tasks"`
	TimeoutTasks     int                 `json:"timeout_tasks"`
	TotalDataSize    int64               `json:"total_data_size_bytes"`
	AvgDuration      int64               `json:"avg_duration_ms"`
	Sessions         []MigrationSession  `json:"sessions"`
}

// MigrationLogger manages migration operation logging
type MigrationLogger struct {
	dataDir        string
	sessionsDir    string
	archiveDir     string
	mu             sync.RWMutex
	activeSessions map[string]*MigrationSession
	retentionDays  int
}

// NewMigrationLogger creates a new migration logger
func NewMigrationLogger(dataDir string, retentionDays int) (*MigrationLogger, error) {
	migrationsDir := filepath.Join(dataDir, "migrations")
	sessionsDir := filepath.Join(migrationsDir, "sessions")
	archiveDir := filepath.Join(migrationsDir, "archive")

	// Create directories
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}
	if err := os.MkdirAll(archiveDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create archive directory: %w", err)
	}

	return &MigrationLogger{
		dataDir:        migrationsDir,
		sessionsDir:    sessionsDir,
		archiveDir:     archiveDir,
		activeSessions: make(map[string]*MigrationSession),
		retentionDays:  retentionDays,
	}, nil
}

// StartSession creates a new migration session
func (ml *MigrationLogger) StartSession(userID, username, ipAddress, userAgent string, totalTasks int, requestJSON string) (*MigrationSession, error) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session := &MigrationSession{
		ID:           uuid.New().String(),
		UserID:       userID,
		Username:     username,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		StartTime:    time.Now(),
		Status:       "running",
		TotalTasks:   totalTasks,
		Tasks:        make([]MigrationTask, 0, totalTasks),
		Metadata:     make(map[string]interface{}),
	}

	// Store the original request JSON
	if requestJSON != "" {
		session.Metadata["request_json"] = requestJSON
	}

	ml.activeSessions[session.ID] = session

	// Save initial session state
	if err := ml.saveSession(session); err != nil {
		return nil, err
	}

	return session, nil
}

// StartTask records the start of a task within a session
func (ml *MigrationLogger) StartTask(sessionID string, taskIndex int, action, sourceURL, targetURL, repoID, graphID string) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session, ok := ml.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	task := MigrationTask{
		TaskIndex: taskIndex,
		Action:    action,
		SourceURL: sourceURL,
		TargetURL: targetURL,
		RepoID:    repoID,
		GraphID:   graphID,
		StartTime: time.Now(),
		Status:    "running",
	}

	session.Tasks = append(session.Tasks, task)

	return ml.saveSession(session)
}

// UpdateTask updates a task with progress information
func (ml *MigrationLogger) UpdateTask(sessionID string, taskIndex int, dataSize, tripleCount int64) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session, ok := ml.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if taskIndex >= len(session.Tasks) {
		return fmt.Errorf("task index %d out of range", taskIndex)
	}

	task := &session.Tasks[taskIndex]
	task.DataSize = dataSize
	task.TripleCount = tripleCount

	return ml.saveSession(session)
}

// CompleteTask marks a task as successfully completed
func (ml *MigrationLogger) CompleteTask(sessionID string, taskIndex int, dataSize, tripleCount int64) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session, ok := ml.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if taskIndex >= len(session.Tasks) {
		return fmt.Errorf("task index %d out of range", taskIndex)
	}

	task := &session.Tasks[taskIndex]
	now := time.Now()
	task.EndTime = &now
	task.Duration = now.Sub(task.StartTime).Milliseconds()
	task.Status = "success"
	task.DataSize = dataSize
	task.TripleCount = tripleCount

	session.CompletedTasks++
	session.TotalDataSize += dataSize

	return ml.saveSession(session)
}

// FailTask marks a task as failed
func (ml *MigrationLogger) FailTask(sessionID string, taskIndex int, errorType, errorMessage string, retryCount int) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session, ok := ml.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if taskIndex >= len(session.Tasks) {
		return fmt.Errorf("task index %d out of range", taskIndex)
	}

	task := &session.Tasks[taskIndex]
	now := time.Now()
	task.EndTime = &now
	task.Duration = now.Sub(task.StartTime).Milliseconds()
	task.Status = "error"
	task.ErrorType = errorType
	task.ErrorMessage = errorMessage
	task.RetryCount = retryCount

	session.FailedTasks++

	return ml.saveSession(session)
}

// TimeoutTask marks a task as timed out
func (ml *MigrationLogger) TimeoutTask(sessionID string, taskIndex int, timeoutDuration time.Duration) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session, ok := ml.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if taskIndex >= len(session.Tasks) {
		return fmt.Errorf("task index %d out of range", taskIndex)
	}

	task := &session.Tasks[taskIndex]
	now := time.Now()
	task.EndTime = &now
	task.Duration = now.Sub(task.StartTime).Milliseconds()
	task.Status = "timeout"
	task.ErrorType = "timeout"
	task.ErrorMessage = fmt.Sprintf("Operation timed out after %v", timeoutDuration)

	session.TimeoutTasks++

	return ml.saveSession(session)
}

// SetTaskFileInfo sets file upload information for a task
func (ml *MigrationLogger) SetTaskFileInfo(sessionID string, taskIndex int, filename, contentType string, size int64, md5Hash string) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session, ok := ml.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if taskIndex >= len(session.Tasks) {
		return fmt.Errorf("task index %d out of range", taskIndex)
	}

	task := &session.Tasks[taskIndex]
	task.FileInfo = &FileInfo{
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		MD5Hash:     md5Hash,
	}

	return ml.saveSession(session)
}

// CompleteSession marks a session as completed
func (ml *MigrationLogger) CompleteSession(sessionID string) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session, ok := ml.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	now := time.Now()
	session.EndTime = &now
	session.Duration = now.Sub(session.StartTime).Milliseconds()

	// Determine final status
	if session.FailedTasks > 0 || session.TimeoutTasks > 0 {
		session.Status = "failed"
	} else {
		session.Status = "completed"
	}

	// Save final state
	if err := ml.saveSession(session); err != nil {
		return err
	}

	// Add to daily summary
	if err := ml.addToDailySummary(session); err != nil {
		return err
	}

	// Remove from active sessions
	delete(ml.activeSessions, sessionID)

	return nil
}

// FailSession marks a session as failed
func (ml *MigrationLogger) FailSession(sessionID string, errorMessage string) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	session, ok := ml.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	now := time.Now()
	session.EndTime = &now
	session.Duration = now.Sub(session.StartTime).Milliseconds()
	session.Status = "failed"
	session.ErrorMessage = errorMessage

	// Save final state
	if err := ml.saveSession(session); err != nil {
		return err
	}

	// Add to daily summary
	if err := ml.addToDailySummary(session); err != nil {
		return err
	}

	// Remove from active sessions
	delete(ml.activeSessions, sessionID)

	return nil
}

// GetSession retrieves a session by ID
func (ml *MigrationLogger) GetSession(sessionID string) (*MigrationSession, error) {
	ml.mu.RLock()
	// Check active sessions first
	if session, ok := ml.activeSessions[sessionID]; ok {
		ml.mu.RUnlock()
		return session, nil
	}
	ml.mu.RUnlock()

	// Load from disk
	sessionFile := filepath.Join(ml.sessionsDir, fmt.Sprintf("%s.json", sessionID))
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	var session MigrationSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return &session, nil
}

// GetDailySummary retrieves the daily summary for a specific date
func (ml *MigrationLogger) GetDailySummary(date string) (*MigrationDailySummary, error) {
	summaryFile := filepath.Join(ml.dataDir, fmt.Sprintf("migration_%s.json", date))

	data, err := os.ReadFile(summaryFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty summary
			return &MigrationDailySummary{
				Date:     date,
				Sessions: []MigrationSession{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read summary: %w", err)
	}

	var summary MigrationDailySummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("failed to parse summary: %w", err)
	}

	return &summary, nil
}

// GetActiveSessions returns all currently active sessions
func (ml *MigrationLogger) GetActiveSessions() []*MigrationSession {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	sessions := make([]*MigrationSession, 0, len(ml.activeSessions))
	for _, session := range ml.activeSessions {
		sessions = append(sessions, session)
	}

	// Sort by start time
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	return sessions
}

// saveSession persists a session to disk
func (ml *MigrationLogger) saveSession(session *MigrationSession) error {
	sessionFile := filepath.Join(ml.sessionsDir, fmt.Sprintf("%s.json", session.ID))
	tempFile := sessionFile + ".tmp"

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempFile, sessionFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// addToDailySummary adds a completed session to the daily summary
func (ml *MigrationLogger) addToDailySummary(session *MigrationSession) error {
	date := session.StartTime.Format("2006-01-02")
	summaryFile := filepath.Join(ml.dataDir, fmt.Sprintf("migration_%s.json", date))
	lockFile := filepath.Join(ml.dataDir, ".migrations.lock")

	// Acquire lock
	lock := flock.New(lockFile)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	// Load existing summary
	summary, err := ml.GetDailySummary(date)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if summary == nil {
		summary = &MigrationDailySummary{
			Date:     date,
			Sessions: []MigrationSession{},
		}
	}

	// Add session to summary
	summary.Sessions = append(summary.Sessions, *session)
	summary.TotalSessions++
	summary.TotalTasks += session.TotalTasks
	summary.CompletedTasks += session.CompletedTasks
	summary.FailedTasks += session.FailedTasks
	summary.TimeoutTasks += session.TimeoutTasks
	summary.TotalDataSize += session.TotalDataSize

	switch session.Status {
	case "completed":
		summary.CompletedSessions++
	case "failed":
		summary.FailedSessions++
	case "running":
		summary.RunningSessions++
	}

	// Calculate average duration
	var totalDuration int64
	for _, s := range summary.Sessions {
		totalDuration += s.Duration
	}
	if len(summary.Sessions) > 0 {
		summary.AvgDuration = totalDuration / int64(len(summary.Sessions))
	}

	// Save summary
	tempFile := summaryFile + ".tmp"
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempFile, summaryFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// RotateOldLogs implements a sophisticated log rotation strategy:
// - Keep daily logs for 7 days (uncompressed)
// - Compress daily logs older than 7 days into weekly archives
// - Keep weekly archives for 4 weeks
// - Delete archives older than 4 weeks
func (ml *MigrationLogger) RotateOldLogs() error {
	now := time.Now()

	// Step 1: Compress daily logs older than 7 days into weekly archives
	sevenDaysAgo := now.AddDate(0, 0, -7)
	fourWeeksAgo := now.AddDate(0, 0, -28)

	files, err := filepath.Glob(filepath.Join(ml.dataDir, "migration_*.json"))
	if err != nil {
		return fmt.Errorf("failed to list log files: %w", err)
	}

	// Group files by week for compression
	weeklyLogs := make(map[string][]string) // week start date -> list of files

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		fileDate := info.ModTime()

		// If file is older than 7 days but newer than 4 weeks, compress into weekly archive
		if fileDate.Before(sevenDaysAgo) && fileDate.After(fourWeeksAgo) {
			// Get the Monday of that week (ISO week)
			year, week := fileDate.ISOWeek()
			weekKey := fmt.Sprintf("%d-W%02d", year, week)
			weeklyLogs[weekKey] = append(weeklyLogs[weekKey], file)
		}

		// If file is older than 4 weeks, it will be in a weekly archive to delete
	}

	// Step 2: Create/update weekly archives
	for weekKey, dailyFiles := range weeklyLogs {
		weeklyArchive := filepath.Join(ml.archiveDir, fmt.Sprintf("migration_%s.tar.gz", weekKey))

		// Compress all daily logs for this week into one weekly archive
		if err := compressFiles(weeklyArchive, dailyFiles); err != nil {
			fmt.Printf("Warning: failed to compress weekly archive %s: %v\n", weekKey, err)
			continue
		}

		// After successful compression, remove the daily files
		for _, dailyFile := range dailyFiles {
			if err := os.Remove(dailyFile); err != nil {
				fmt.Printf("Warning: failed to remove daily log %s: %v\n", dailyFile, err)
			}
		}
	}

	// Step 3: Delete weekly archives older than 4 weeks
	weeklyArchives, err := filepath.Glob(filepath.Join(ml.archiveDir, "migration_*-W*.tar.gz"))
	if err != nil {
		return fmt.Errorf("failed to list weekly archives: %w", err)
	}

	for _, archive := range weeklyArchives {
		info, err := os.Stat(archive)
		if err != nil {
			continue
		}

		if info.ModTime().Before(fourWeeksAgo) {
			if err := os.Remove(archive); err != nil {
				fmt.Printf("Warning: failed to remove old weekly archive %s: %v\n", archive, err)
			} else {
				fmt.Printf("Removed old weekly archive: %s\n", filepath.Base(archive))
			}
		}
	}

	return nil
}

// GetSessionsInDateRange retrieves all sessions within a date range
func (ml *MigrationLogger) GetSessionsInDateRange(startDate, endDate time.Time) ([]MigrationSession, error) {
	sessions := make([]MigrationSession, 0)

	// Iterate through each day in the range
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		summary, err := ml.GetDailySummary(dateStr)
		if err != nil {
			// No data for this date, continue
			continue
		}

		sessions = append(sessions, summary.Sessions...)
	}

	return sessions, nil
}

// MigrationStatistics contains aggregated statistics
type MigrationStatistics struct {
	TotalSessions       int                `json:"total_sessions"`
	CompletedSessions   int                `json:"completed_sessions"`
	FailedSessions      int                `json:"failed_sessions"`
	RunningSessions     int                `json:"running_sessions"`
	TotalTasks          int                `json:"total_tasks"`
	CompletedTasks      int                `json:"completed_tasks"`
	FailedTasks         int                `json:"failed_tasks"`
	TimeoutTasks        int                `json:"timeout_tasks"`
	TotalDataSize       int64              `json:"total_data_size_bytes"`
	TotalDuration       int64              `json:"total_duration_ms"`
	AvgDuration         int64              `json:"avg_duration_ms"`
	AvgTasksPerSession  float64            `json:"avg_tasks_per_session"`
	SuccessRate         float64            `json:"success_rate"`
	ActionCounts        map[string]int     `json:"action_counts"`
	ErrorTypes          map[string]int     `json:"error_types"`
	UserActivity        map[string]int     `json:"user_activity"`
}

// GetStatistics returns aggregated statistics for a date range
func (ml *MigrationLogger) GetStatistics(startDate, endDate time.Time) (*MigrationStatistics, error) {
	stats := &MigrationStatistics{
		ActionCounts: make(map[string]int),
		ErrorTypes:   make(map[string]int),
		UserActivity: make(map[string]int),
	}

	sessions, err := ml.GetSessionsInDateRange(startDate, endDate)
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		stats.TotalSessions++
		stats.TotalTasks += session.TotalTasks
		stats.CompletedTasks += session.CompletedTasks
		stats.FailedTasks += session.FailedTasks
		stats.TimeoutTasks += session.TimeoutTasks
		stats.TotalDataSize += session.TotalDataSize
		stats.TotalDuration += session.Duration

		switch session.Status {
		case "completed":
			stats.CompletedSessions++
		case "failed":
			stats.FailedSessions++
		case "running":
			stats.RunningSessions++
		}

		// Track action types
		for _, task := range session.Tasks {
			stats.ActionCounts[task.Action]++

			// Track errors
			if task.ErrorType != "" {
				stats.ErrorTypes[task.ErrorType]++
			}
		}

		// Track users
		stats.UserActivity[session.Username]++
	}

	// Calculate averages
	if stats.TotalSessions > 0 {
		stats.AvgDuration = stats.TotalDuration / int64(stats.TotalSessions)
		stats.AvgTasksPerSession = float64(stats.TotalTasks) / float64(stats.TotalSessions)
	}
	if stats.TotalTasks > 0 {
		stats.SuccessRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	return stats, nil
}

// compressFiles compresses multiple files into a tar.gz archive
func compressFiles(archivePath string, files []string) error {
	// Create the archive file
	outFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add each file to the archive
	for _, file := range files {
		if err := addFileToTar(tarWriter, file); err != nil {
			return fmt.Errorf("failed to add file %s to archive: %w", file, err)
		}
	}

	return nil
}

// addFileToTar adds a single file to a tar archive
func addFileToTar(tarWriter *tar.Writer, filename string) error {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create tar header
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}

	// Use only the base name in the archive
	header.Name = filepath.Base(filename)

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Copy file content
	if _, err := io.Copy(tarWriter, file); err != nil {
		return err
	}

	return nil
}
