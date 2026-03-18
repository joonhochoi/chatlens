package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	sqlite_vec.Auto()
}

// Open initializes (or opens) the SQLite database at dbPath and applies the schema.
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1) // sqlite doesn't support concurrent writers

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

// IsFileProcessed checks whether a file with the given SHA256 hash has been imported.
func IsFileProcessed(db *sql.DB, hash string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM processed_files WHERE file_hash = ?`, hash,
	).Scan(&count)
	return count > 0, err
}

// InsertChunks persists chunks, their embeddings, and message hashes in a single transaction.
// embeddings[i] corresponds to chunks[i]; both slices must have the same length.
// msgHashSources are the original messages whose hashes should be recorded to prevent future duplicates.
func InsertChunks(db *sql.DB, fileHash, fileName string, chunks []ChunkRow, embeddings [][]float32, msgHashSources []MessageForDedup) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Register the file
	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO processed_files (file_name, file_hash) VALUES (?, ?)`,
		fileName, fileHash,
	); err != nil {
		return fmt.Errorf("insert processed_files: %w", err)
	}

	for i, c := range chunks {
		res, err := tx.Exec(
			`INSERT INTO chunks (text, is_leader, start_time, end_time, file_hash)
			 VALUES (?, ?, ?, ?, ?)`,
			c.Text,
			boolToInt(c.IsLeader),
			c.StartTime.Format(time.RFC3339),
			c.EndTime.Format(time.RFC3339),
			fileHash,
		)
		if err != nil {
			return fmt.Errorf("insert chunk %d: %w", i, err)
		}
		chunkID, err := res.LastInsertId()
		if err != nil {
			return err
		}

		vecBytes := float32SliceToBytes(embeddings[i])
		if _, err := tx.Exec(
			`INSERT INTO chunk_vectors (chunk_id, embedding) VALUES (?, ?)`,
			chunkID, vecBytes,
		); err != nil {
			return fmt.Errorf("insert chunk_vectors %d: %w", i, err)
		}
	}

	// Record message hashes to prevent future duplicates
	if err := InsertMessageHashes(tx, msgHashSources); err != nil {
		return fmt.Errorf("insert message_hashes: %w", err)
	}

	// Store raw messages for the chat history view
	if err := insertMessages(tx, msgHashSources); err != nil {
		return fmt.Errorf("insert messages: %w", err)
	}

	return tx.Commit()
}

// GetProcessedFileNames returns all imported file names.
func GetProcessedFileNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT file_name FROM processed_files ORDER BY imported_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// MessageForDedup represents a parsed message used for deduplication.
type MessageForDedup struct {
	Timestamp time.Time
	Speaker   string
	Content   string
}

// messageHash returns a deterministic hash for a single message.
func messageHash(m MessageForDedup) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s", m.Timestamp.UTC().Format(time.RFC3339), m.Speaker, m.Content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// FilterNewMessages returns only the messages whose hashes are not yet in message_hashes.
func FilterNewMessages(db *sql.DB, msgs []MessageForDedup) ([]MessageForDedup, error) {
	if len(msgs) == 0 {
		return nil, nil
	}

	// Load existing hashes into a set for fast lookup
	rows, err := db.Query(`SELECT msg_hash FROM message_hashes`)
	if err != nil {
		return nil, fmt.Errorf("query message_hashes: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]struct{})
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		existing[h] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var newMsgs []MessageForDedup
	for _, m := range msgs {
		if _, seen := existing[messageHash(m)]; !seen {
			newMsgs = append(newMsgs, m)
		}
	}
	return newMsgs, nil
}

// InsertMessageHashes stores hashes for the given messages (idempotent via INSERT OR IGNORE).
func InsertMessageHashes(tx *sql.Tx, msgs []MessageForDedup) error {
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO message_hashes (msg_hash) VALUES (?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range msgs {
		if _, err := stmt.Exec(messageHash(m)); err != nil {
			return err
		}
	}
	return nil
}

// insertMessages stores raw messages into the messages table (for chat history view).
func insertMessages(tx *sql.Tx, msgs []MessageForDedup) error {
	stmt, err := tx.Prepare(`INSERT INTO messages (timestamp, speaker, content) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range msgs {
		if _, err := stmt.Exec(m.Timestamp.Format(time.RFC3339), m.Speaker, m.Content); err != nil {
			return err
		}
	}
	return nil
}

// RawMessage is a single stored message retrieved from the messages table.
type RawMessage struct {
	Timestamp time.Time
	Speaker   string
	Content   string
}

// GetChatDates returns the distinct dates (YYYY-MM-DD) for which messages exist, sorted ascending.
func GetChatDates(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT date(timestamp) FROM messages ORDER BY date(timestamp)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// GetMessagesByDate returns all messages for a given date string ("YYYY-MM-DD"), ordered by time.
func GetMessagesByDate(db *sql.DB, date string) ([]RawMessage, error) {
	rows, err := db.Query(
		`SELECT timestamp, speaker, content FROM messages WHERE date(timestamp) = ? ORDER BY timestamp`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []RawMessage
	for rows.Next() {
		var tsStr, speaker, content string
		if err := rows.Scan(&tsStr, &speaker, &content); err != nil {
			return nil, err
		}
		ts, parseErr := time.Parse(time.RFC3339, tsStr)
		if parseErr != nil {
			ts, _ = time.Parse("2006-01-02 15:04:05", tsStr)
		}
		msgs = append(msgs, RawMessage{Timestamp: ts, Speaker: speaker, Content: content})
	}
	return msgs, rows.Err()
}

const KeywordPageSize = 10

// KeywordMsg is a single message returned by keyword search.
type KeywordMsg struct {
	Timestamp time.Time
	Speaker   string
	Content   string
}

// SearchMessages returns messages whose content matches the keyword, newest first.
// offset is the number of rows to skip (for pagination).
// startDate and endDate are optional "YYYY-MM-DD" strings; empty means no filter.
// Also returns the total match count.
func SearchMessages(db *sql.DB, keyword string, offset int, startDate, endDate string) ([]KeywordMsg, int, error) {
	like := "%" + keyword + "%"

	var conds []string
	var baseArgs []interface{}
	conds = append(conds, "content LIKE ?")
	baseArgs = append(baseArgs, like)
	if startDate != "" {
		conds = append(conds, "date(timestamp) >= ?")
		baseArgs = append(baseArgs, startDate)
	}
	if endDate != "" {
		conds = append(conds, "date(timestamp) <= ?")
		baseArgs = append(baseArgs, endDate)
	}
	whereSQL := strings.Join(conds, " AND ")

	var total int
	countArgs := append([]interface{}{}, baseArgs...)
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM messages WHERE "+whereSQL, countArgs...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	selectArgs := append([]interface{}{}, baseArgs...)
	selectArgs = append(selectArgs, KeywordPageSize, offset)
	rows, err := db.Query(
		"SELECT timestamp, speaker, content FROM messages WHERE "+whereSQL+" ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		selectArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var msgs []KeywordMsg
	for rows.Next() {
		var tsStr, speaker, content string
		if err := rows.Scan(&tsStr, &speaker, &content); err != nil {
			return nil, 0, err
		}
		ts, parseErr := time.Parse(time.RFC3339, tsStr)
		if parseErr != nil {
			ts, _ = time.Parse("2006-01-02 15:04:05", tsStr)
		}
		msgs = append(msgs, KeywordMsg{Timestamp: ts, Speaker: speaker, Content: content})
	}
	return msgs, total, rows.Err()
}

// DeleteAllData removes all imported data (chunks, vectors, files, message hashes, messages).
func DeleteAllData(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range []string{
		`DELETE FROM chunk_vectors`,
		`DELETE FROM chunks`,
		`DELETE FROM processed_files`,
		`DELETE FROM message_hashes`,
		`DELETE FROM messages`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("delete (%s): %w", stmt, err)
		}
	}
	return tx.Commit()
}

// ChunkRow is the data needed to insert a chunk into the DB.
type ChunkRow struct {
	Text      string
	IsLeader  bool
	StartTime time.Time
	EndTime   time.Time
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Float32ToBytes converts []float32 to little-endian bytes for sqlite-vec.
func Float32ToBytes(vec []float32) []byte {
	return float32SliceToBytes(vec)
}

// float32SliceToBytes converts []float32 to little-endian bytes for sqlite-vec.
func float32SliceToBytes(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}
