// Package db handles SQLite database operations for rating persistence.
package db

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yourusername/openskill-rating-bot/internal/rating"
)

// Store manages persistent player ratings in SQLite.
type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// PlayerRecord represents a player's stored data.
type PlayerRecord struct {
	UserID    string
	Mu        float64
	Sigma     float64
	Wins      int
	Losses    int
	UpdatedAt time.Time
}

// MatchRecord represents a recorded match result.
type MatchRecord struct {
	ID        int64
	WinnerID  string
	LoserID   string
	CreatedAt time.Time
}

// New creates and initializes a new SQLite store.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	log.Printf("Database initialized at %s", dbPath)
	return store, nil
}

// migrate creates tables if they don't exist.
func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS players (
			user_id    TEXT PRIMARY KEY,
			mu         REAL NOT NULL DEFAULT 1000.0,
			sigma      REAL NOT NULL DEFAULT 333.3333333333333,
			wins       INTEGER NOT NULL DEFAULT 0,
			losses     INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS matches (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			winner_id  TEXT NOT NULL,
			loser_id   TEXT NOT NULL,
			winner_mu_before  REAL NOT NULL,
			winner_mu_after   REAL NOT NULL,
			loser_mu_before   REAL NOT NULL,
			loser_mu_after    REAL NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_matches_winner ON matches(winner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_matches_loser ON matches(loser_id)`,
		`CREATE INDEX IF NOT EXISTS idx_matches_created ON matches(created_at)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migration query failed: %w\nQuery: %s", err, q)
		}
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// GetPlayer retrieves a player's record, creating one with defaults if not found.
func (s *Store) GetPlayer(userID string) (*PlayerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(
		`SELECT user_id, mu, sigma, wins, losses, updated_at FROM players WHERE user_id = ?`,
		userID,
	)

	var p PlayerRecord
	err := row.Scan(&p.UserID, &p.Mu, &p.Sigma, &p.Wins, &p.Losses, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		// Return default rating
		return &PlayerRecord{
			UserID:    userID,
			Mu:        rating.DefaultMu,
			Sigma:     rating.DefaultSigma,
			Wins:      0,
			Losses:    0,
			UpdatedAt: time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get player %s: %w", userID, err)
	}
	return &p, nil
}

// UpsertPlayer inserts or updates a player record.
func (s *Store) UpsertPlayer(userID string, r rating.Rating, wins, losses int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO players (user_id, mu, sigma, wins, losses, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET
		   mu = excluded.mu,
		   sigma = excluded.sigma,
		   wins = excluded.wins,
		   losses = excluded.losses,
		   updated_at = CURRENT_TIMESTAMP`,
		userID, r.Mu, r.Sigma, wins, losses,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert player %s: %w", userID, err)
	}
	return nil
}

// RecordMatch stores a match result and updates both players' ratings atomically.
func (s *Store) RecordMatch(winnerID, loserID string, winnerBefore, winnerAfter, loserBefore, loserAfter rating.Rating) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert match record
	_, err = tx.Exec(
		`INSERT INTO matches (winner_id, loser_id, winner_mu_before, winner_mu_after, loser_mu_before, loser_mu_after, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		winnerID, loserID,
		winnerBefore.Mu, winnerAfter.Mu,
		loserBefore.Mu, loserAfter.Mu,
	)
	if err != nil {
		return fmt.Errorf("failed to insert match: %w", err)
	}

	// Get current win/loss counts for winner
	var winnerWins, winnerLosses int
	row := tx.QueryRow(`SELECT COALESCE(wins,0), COALESCE(losses,0) FROM players WHERE user_id = ?`, winnerID)
	if err := row.Scan(&winnerWins, &winnerLosses); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get winner stats: %w", err)
	}
	winnerWins++

	// Get current win/loss counts for loser
	var loserWins, loserLosses int
	row = tx.QueryRow(`SELECT COALESCE(wins,0), COALESCE(losses,0) FROM players WHERE user_id = ?`, loserID)
	if err := row.Scan(&loserWins, &loserLosses); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get loser stats: %w", err)
	}
	loserLosses++

	// Update winner
	_, err = tx.Exec(
		`INSERT INTO players (user_id, mu, sigma, wins, losses, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET
		   mu = excluded.mu,
		   sigma = excluded.sigma,
		   wins = excluded.wins,
		   losses = excluded.losses,
		   updated_at = CURRENT_TIMESTAMP`,
		winnerID, winnerAfter.Mu, winnerAfter.Sigma, winnerWins, winnerLosses,
	)
	if err != nil {
		return fmt.Errorf("failed to update winner: %w", err)
	}

	// Update loser
	_, err = tx.Exec(
		`INSERT INTO players (user_id, mu, sigma, wins, losses, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET
		   mu = excluded.mu,
		   sigma = excluded.sigma,
		   wins = excluded.wins,
		   losses = excluded.losses,
		   updated_at = CURRENT_TIMESTAMP`,
		loserID, loserAfter.Mu, loserAfter.Sigma, loserWins, loserLosses,
	)
	if err != nil {
		return fmt.Errorf("failed to update loser: %w", err)
	}

	return tx.Commit()
}

// GetRanking returns all players sorted by ordinal rating descending.
func (s *Store) GetRanking(limit int) ([]*PlayerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(
		`SELECT user_id, mu, sigma, wins, losses, updated_at
		 FROM players
		 ORDER BY (mu - 3 * sigma) DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query ranking: %w", err)
	}
	defer rows.Close()

	var players []*PlayerRecord
	for rows.Next() {
		var p PlayerRecord
		if err := rows.Scan(&p.UserID, &p.Mu, &p.Sigma, &p.Wins, &p.Losses, &p.UpdatedAt); err != nil {
			return nil, err
		}
		players = append(players, &p)
	}
	return players, rows.Err()
}

// GetMatchHistory returns recent matches for a user.
func (s *Store) GetMatchHistory(userID string, limit int) ([]*MatchRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query(
		`SELECT id, winner_id, loser_id, created_at
		 FROM matches
		 WHERE winner_id = ? OR loser_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		userID, userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query match history: %w", err)
	}
	defer rows.Close()

	var matches []*MatchRecord
	for rows.Next() {
		var m MatchRecord
		if err := rows.Scan(&m.ID, &m.WinnerID, &m.LoserID, &m.CreatedAt); err != nil {
			return nil, err
		}
		matches = append(matches, &m)
	}
	return matches, rows.Err()
}
