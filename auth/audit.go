package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	Action      string                 `json:"action"`
	Resource    string                 `json:"resource"`
	Success     bool                   `json:"success"`
	IPAddress   string                 `json:"ip_address"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	ErrorMsg    string                 `json:"error_message,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// AuditLog represents a day's worth of audit entries
type AuditLog struct {
	Date    string        `json:"date"` // YYYY-MM-DD format
	Entries []AuditEntry  `json:"entries"`
}

// AuditLogger handles audit log storage and rotation
type AuditLogger struct {
	dataDir  string
	mutex    sync.RWMutex
	lockFile *flock.Flock
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(dataDir string) (*AuditLogger, error) {
	auditDir := filepath.Join(dataDir, "audit")

	// Create audit directory if it doesn't exist
	if err := os.MkdirAll(auditDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit directory: %w", err)
	}

	// Create lock file
	lockPath := filepath.Join(auditDir, ".audit.lock")
	lockFile := flock.New(lockPath)

	return &AuditLogger{
		dataDir:  auditDir,
		lockFile: lockFile,
	}, nil
}

// LogEntry writes an audit entry to the log
func (l *AuditLogger) LogEntry(entry AuditEntry) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Acquire lock
	locked, err := l.lockFile.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("could not acquire lock")
	}
	defer l.lockFile.Unlock()

	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Get today's log file
	date := entry.Timestamp.Format("2006-01-02")
	logFile := filepath.Join(l.dataDir, fmt.Sprintf("audit_%s.json", date))

	// Load existing log or create new one
	var log AuditLog
	if data, err := os.ReadFile(logFile); err == nil {
		if err := json.Unmarshal(data, &log); err != nil {
			return fmt.Errorf("failed to parse existing log: %w", err)
		}
	} else {
		log = AuditLog{
			Date:    date,
			Entries: []AuditEntry{},
		}
	}

	// Add new entry
	log.Entries = append(log.Entries, entry)

	// Write to temp file first (atomic write)
	tempFile := logFile + ".tmp"
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, logFile); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// GetEntriesForDate retrieves all audit entries for a specific date
func (l *AuditLogger) GetEntriesForDate(date string) ([]AuditEntry, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	logFile := filepath.Join(l.dataDir, fmt.Sprintf("audit_%s.json", date))

	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	var log AuditLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("failed to parse log file: %w", err)
	}

	return log.Entries, nil
}

// GetEntriesRange retrieves all audit entries within a date range
func (l *AuditLogger) GetEntriesRange(startDate, endDate string) ([]AuditEntry, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	var allEntries []AuditEntry

	// Iterate through each day in the range
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		dateStr := date.Format("2006-01-02")
		entries, err := l.GetEntriesForDate(dateStr)
		if err != nil {
			// Skip missing files
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

// GetRecentEntries retrieves the most recent audit entries
func (l *AuditLogger) GetRecentEntries(limit int) ([]AuditEntry, error) {
	// Get entries for the last 7 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -7)

	entries, err := l.GetEntriesRange(
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}

	// Return most recent entries up to limit
	if len(entries) > limit {
		return entries[len(entries)-limit:], nil
	}

	return entries, nil
}

// ListLogFiles returns all audit log files
func (l *AuditLogger) ListLogFiles() ([]string, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	pattern := filepath.Join(l.dataDir, "audit_*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list log files: %w", err)
	}

	return files, nil
}

// RotateOldLogs compresses and archives logs older than a certain number of days
func (l *AuditLogger) RotateOldLogs(daysToKeep int) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep)

	files, err := l.ListLogFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		// Extract date from filename
		base := filepath.Base(file)
		if len(base) < 17 { // "audit_YYYY-MM-DD.json"
			continue
		}
		dateStr := base[6:16] // Extract YYYY-MM-DD

		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// If file is older than cutoff, compress and archive
		if fileDate.Before(cutoffDate) {
			// For now, just delete old files
			// In a full implementation, this would compress to .gz
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("failed to remove old log file: %w", err)
			}
		}
	}

	return nil
}

// SearchEntries searches for audit entries matching criteria
func (l *AuditLogger) SearchEntries(criteria AuditSearchCriteria) ([]AuditEntry, error) {
	// Get entries for the date range
	entries, err := l.GetEntriesRange(criteria.StartDate, criteria.EndDate)
	if err != nil {
		return nil, err
	}

	// Filter entries based on criteria
	var filtered []AuditEntry
	for _, entry := range entries {
		if matchesCriteria(entry, criteria) {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

// AuditSearchCriteria defines search parameters for audit logs
type AuditSearchCriteria struct {
	StartDate string   // YYYY-MM-DD
	EndDate   string   // YYYY-MM-DD
	Username  string   // Filter by username
	Action    string   // Filter by action
	Resource  string   // Filter by resource
	Success   *bool    // Filter by success (nil = all)
	IPAddress string   // Filter by IP address
}

// matchesCriteria checks if an entry matches the search criteria
func matchesCriteria(entry AuditEntry, criteria AuditSearchCriteria) bool {
	if criteria.Username != "" && entry.Username != criteria.Username {
		return false
	}

	if criteria.Action != "" && entry.Action != criteria.Action {
		return false
	}

	if criteria.Resource != "" && entry.Resource != criteria.Resource {
		return false
	}

	if criteria.Success != nil && entry.Success != *criteria.Success {
		return false
	}

	if criteria.IPAddress != "" && entry.IPAddress != criteria.IPAddress {
		return false
	}

	return true
}
