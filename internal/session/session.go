package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session represents the current IPCrawler session state
type Session struct {
	Target       string    `json:"target"`
	LastModified time.Time `json:"last_modified"`
	OutputDir    string    `json:"output_dir"`
	SessionID    string    `json:"session_id"`
}

// Manager handles session persistence
type Manager struct {
	sessionFile string
}

// NewManager creates a new session manager
func NewManager() *Manager {
	// Get session file path - try multiple locations
	sessionFile := getSessionFilePath()
	return &Manager{
		sessionFile: sessionFile,
	}
}

// getSessionFilePath determines the best location for the session file
func getSessionFilePath() string {
	// Try current directory first
	if _, err := os.Stat(".ipcrawler_session"); err == nil {
		return ".ipcrawler_session"
	}
	
	// Try home directory
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".ipcrawler_session")
	}
	
	// Fallback to current directory
	return ".ipcrawler_session"
}

// Load loads the session from disk
func (m *Manager) Load() (*Session, error) {
	data, err := os.ReadFile(m.sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No session file exists yet
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}
	
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}
	
	return &session, nil
}

// Save saves the session to disk
func (m *Manager) Save(session *Session) error {
	session.LastModified = time.Now()
	
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	
	if err := os.WriteFile(m.sessionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	
	return nil
}

// Update updates specific session fields
func (m *Manager) Update(updates map[string]interface{}) error {
	// Load existing session or create new one
	session, err := m.Load()
	if err != nil {
		return err
	}
	if session == nil {
		session = &Session{
			SessionID: generateSessionID(),
		}
	}
	
	// Apply updates
	for key, value := range updates {
		switch key {
		case "target":
			if v, ok := value.(string); ok {
				session.Target = v
			}
		case "output_dir":
			if v, ok := value.(string); ok {
				session.OutputDir = v
			}
		case "session_id":
			if v, ok := value.(string); ok {
				session.SessionID = v
			}
		}
	}
	
	return m.Save(session)
}

// Clear removes the session file
func (m *Manager) Clear() error {
	if err := os.Remove(m.sessionFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear session: %w", err)
	}
	return nil
}

// GetSessionFile returns the path to the session file
func (m *Manager) GetSessionFile() string {
	return m.sessionFile
}

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().Unix())
}