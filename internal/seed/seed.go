package seed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// UserRecord is a single seed user, persisted in local storage
// (data/seed/users.json) so that demo credentials are not hardcoded in Go source.
type UserRecord struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	Role         string `json:"role"`
	IsActive     bool   `json:"is_active"`
}

// SongRecord describes a seed song to load from local storage
// (data/seed/songs.json). The referenced MIDI and structures CSV files are
// themselves local files.
type SongRecord struct {
	Title         string `json:"title"`
	Artist        string `json:"artist"`
	MIDIPath      string `json:"midi_path"`
	StructuresCSV string `json:"structures_csv"`
}

// Dir returns the seed data directory. It honors the SEED_DIR environment
// variable when set, otherwise defaulting to <repo>/data/seed resolved from
// this source file's location so it is independent of the working directory.
func Dir() string {
	if d := os.Getenv("SEED_DIR"); d != "" {
		if abs, err := filepath.Abs(d); err == nil {
			return abs
		}
		return d
	}
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Join(repoRoot, "data", "seed")
}

// RepoRoot returns the repository root, resolved the same way as Dir.
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// UsersFile is the default path for the seed users data file.
func UsersFile() string { return filepath.Join(Dir(), "users.json") }

// SongsFile is the default path for the seed songs data file.
func SongsFile() string { return filepath.Join(Dir(), "songs.json") }

// LoadUsers reads seed users from the given JSON file.
func LoadUsers(path string) ([]UserRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var recs []UserRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

// LoadSongs reads seed songs from the given JSON file.
func LoadSongs(path string) ([]SongRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var recs []SongRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}
