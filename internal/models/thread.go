package models

import (
	"database/sql"
	"fmt"
	"garden-onion/internal/database"
	"time"
)

type Thread struct {
	ID           int
	Title        string
	Content      string
	AuthorName   string
	Category     string
	ParentID     sql.NullInt64
	ParentTitle  string
	ParentAuthor string
	SignalScore  int
	IsLocked     bool
	CreatedAt    time.Time
	PublishedAt  time.Time
}

type TagStat struct {
	Category string
	Count    int
}

func GetDynamicThreshold(category string) int {
	var avg sql.NullFloat64
	query := `SELECT AVG(vote_count) FROM (SELECT COUNT(*) as vote_count FROM thread_tags tt JOIN threads t ON tt.thread_id = t.id WHERE t.category = $1 AND tt.suggested_category = $2 GROUP BY tt.thread_id) sub`
	err := database.DB.QueryRow(query, category, category).Scan(&avg)
	if err != nil || !avg.Valid || avg.Float64 < 1 {
		return 1
	}
	return int(avg.Float64)
}

func GetAllThreads(category string) ([]Thread, error) {
	var rows *sql.Rows
	var err error
	now := time.Now().UTC()
	baseQuery := `
		SELECT t.id, t.title, t.content, t.author_name, t.category, t.parent_id, p.title as parent_title, t.parent_author, t.signal_score, t.is_locked, t.created_at, t.published_at,
		CASE WHEN t.id IN (SELECT id FROM threads WHERE is_hidden = FALSE AND published_at <= $1 ORDER BY published_at DESC LIMIT 7) THEN 1 ELSE 0 END as is_fresh,
		(EXTRACT(EPOCH FROM t.published_at) + (t.signal_score * 3600)) as rank_score
		FROM threads t 
		LEFT JOIN threads p ON t.parent_id = p.id 
		WHERE t.is_hidden = FALSE AND t.published_at <= $2`
	orderClause := " ORDER BY is_fresh DESC, rank_score DESC, t.published_at DESC"

	if category == "feed" || category == "" {
		rows, err = database.DB.Query(baseQuery+orderClause, now, now)
	} else if category == "top" {
		return GetTopThreads(10)
	} else {
		rows, err = database.DB.Query(baseQuery+" AND t.category = $3"+orderClause, now, now, category)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var threads []Thread
	for rows.Next() {
		var t Thread
		var pt, pa sql.NullString
		var fresh int
		var rs float64
		err := rows.Scan(&t.ID, &t.Title, &t.Content, &t.AuthorName, &t.Category, &t.ParentID, &pt, &pa, &t.SignalScore, &t.IsLocked, &t.CreatedAt, &t.PublishedAt, &fresh, &rs)
		if err != nil {
			return nil, err
		}
		if pt.Valid {
			t.ParentTitle = pt.String
		}
		if pa.Valid {
			t.ParentAuthor = pa.String
		}
		threads = append(threads, t)
	}
	return threads, nil
}

func GetTopThreads(threshold int) ([]Thread, error) {
	now := time.Now().UTC()
	query := `
		SELECT t.id, t.title, t.content, t.author_name, t.category, t.parent_id, p.title as parent_title, t.parent_author, t.signal_score, t.is_locked, t.created_at, t.published_at 
		FROM threads t 
		LEFT JOIN threads p ON t.parent_id = p.id 
		WHERE t.is_hidden = FALSE AND (t.signal_score >= $1 OR t.category = 'signal') AND t.published_at <= $2 
		ORDER BY t.signal_score DESC, t.published_at DESC`
	rows, err := database.DB.Query(query, threshold, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var threads []Thread
	for rows.Next() {
		var t Thread
		var pt, pa sql.NullString
		err := rows.Scan(&t.ID, &t.Title, &t.Content, &t.AuthorName, &t.Category, &t.ParentID, &pt, &pa, &t.SignalScore, &t.IsLocked, &t.CreatedAt, &t.PublishedAt)
		if err != nil {
			return nil, err
		}
		if pt.Valid {
			t.ParentTitle = pt.String
		}
		if pa.Valid {
			t.ParentAuthor = pa.String
		}
		threads = append(threads, t)
	}
	return threads, nil
}

func GetResponses(parentID int) ([]Thread, error) {
	now := time.Now().UTC()
	rows, err := database.DB.Query("SELECT id, title, author_name, category, signal_score, created_at, published_at FROM threads WHERE parent_id = $1 AND is_hidden = FALSE AND published_at <= $2 ORDER BY published_at ASC", parentID, now)
	if err != nil { return nil, err }
	defer rows.Close()
	var threads []Thread
	for rows.Next() {
		var t Thread
		err := rows.Scan(&t.ID, &t.Title, &t.AuthorName, &t.Category, &t.SignalScore, &t.CreatedAt, &t.PublishedAt)
		if err != nil { return nil, err }
		threads = append(threads, t)
	}
	return threads, nil
}

func SuggestTag(threadID int, visitorID string, category string) error {
	if category == "" || category == "feed" { return nil }
	var isLocked bool
	_ = database.DB.QueryRow("SELECT is_locked FROM threads WHERE id = $1", threadID).Scan(&isLocked)
	_, err := database.DB.Exec("INSERT INTO thread_tags (thread_id, visitor_id, suggested_category) VALUES ($1, $2, $3) ON CONFLICT(thread_id, visitor_id) DO UPDATE SET suggested_category=EXCLUDED.suggested_category", threadID, visitorID, category)
	if err != nil { return err }
	if isLocked { return nil }
	var count int
	_ = database.DB.QueryRow("SELECT COUNT(1) FROM thread_tags WHERE thread_id = $1 AND suggested_category = $2", threadID, category).Scan(&count)
	threshold := GetDynamicThreshold(category)
	if count >= threshold {
		_, _ = database.DB.Exec("UPDATE threads SET category = $1, is_locked = TRUE WHERE id = $2", category, threadID)
	}
	return nil
}

func GetThreadTagStats(threadID int) ([]TagStat, error) {
	rows, err := database.DB.Query("SELECT suggested_category, COUNT(1) FROM thread_tags WHERE thread_id = $1 GROUP BY suggested_category ORDER BY COUNT(1) DESC", threadID)
	if err != nil { return nil, err }
	defer rows.Close()
	var stats []TagStat
	for rows.Next() {
		var s TagStat
		if err := rows.Scan(&s.Category, &s.Count); err == nil { stats = append(stats, s) }
	}
	return stats, nil
}

func GetPendingCount() int {
	now := time.Now().UTC()
	var count int
	_ = database.DB.QueryRow("SELECT COUNT(1) FROM threads WHERE published_at > $1", now).Scan(&count)
	return count
}

func SaveThreadWithSchedule(title, content, authorName, category string, parentID int, parentAuthor string, publishedAt time.Time) error {
	if authorName == "" { authorName = "@anonymous" }
	if category == "" { category = "feed" }
	var pID sql.NullInt64
	if parentID > 0 { pID = sql.NullInt64{Int64: int64(parentID), Valid: true} }
	_, err := database.DB.Exec(`
		INSERT INTO threads (title, content, author_name, category, parent_id, parent_author, signal_score, is_locked, published_at) 
		VALUES ($1, $2, $3, $4, $5, $6, 0, FALSE, $7)`, 
		title, content, authorName, category, pID, parentAuthor, publishedAt)
	return err
}

func GetThreadByID(id int) (*Thread, error) {
	var t Thread
	var pt, pa sql.NullString
	err := database.DB.QueryRow(`
		SELECT t.id, t.title, t.content, t.author_name, t.category, t.parent_id, p.title as parent_title, t.parent_author, t.signal_score, t.is_locked, t.created_at, t.published_at 
		FROM threads t 
		LEFT JOIN threads p ON t.parent_id = p.id 
		WHERE t.id = $1`, id).
		Scan(&t.ID, &t.Title, &t.Content, &t.AuthorName, &t.Category, &t.ParentID, &pt, &pa, &t.SignalScore, &t.IsLocked, &t.CreatedAt, &t.PublishedAt)
	if err != nil { return nil, err }
	if pt.Valid { t.ParentTitle = pt.String }
	if pa.Valid { t.ParentAuthor = pa.String }
	return &t, nil
}

func ToggleVote(threadID int, visitorID string, voteType int) error {
	tx, err := database.DB.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	var currentVote int
	err = tx.QueryRow("SELECT vote_type FROM votes WHERE thread_id = $1 AND visitor_id = $2", threadID, visitorID).Scan(&currentVote)
	
	if err == nil {
		if currentVote == voteType {
			tx.Exec("DELETE FROM votes WHERE thread_id = $1 AND visitor_id = $2", threadID, visitorID)
			tx.Exec("UPDATE threads SET signal_score = signal_score - $1 WHERE id = $2", currentVote, threadID)
		} else {
			tx.Exec("UPDATE votes SET vote_type = $1 WHERE thread_id = $2 AND visitor_id = $3", voteType, threadID, visitorID)
			tx.Exec("UPDATE threads SET signal_score = signal_score + $1 WHERE id = $2", voteType*2, threadID)
		}
	} else {
		tx.Exec("INSERT INTO votes (thread_id, visitor_id, vote_type) VALUES ($1, $2, $3)", threadID, visitorID, voteType)
		tx.Exec("UPDATE threads SET signal_score = signal_score + $1 WHERE id = $2", voteType, threadID)
	}

	if voteType == 1 {
		var newScore int
		var currentCategory string
		tx.QueryRow("SELECT signal_score, category FROM threads WHERE id = $1", threadID).Scan(&newScore, &currentCategory)

		if newScore > 0 && newScore%10 == 0 {
			systemVisitorID := fmt.Sprintf("__system_consensus_%d", newScore)
			_, _ = tx.Exec("INSERT INTO thread_tags (thread_id, visitor_id, suggested_category) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", threadID, systemVisitorID, currentCategory)
		}
	}
	
	return tx.Commit()
}

func GetVisitorVotes(visitorID string) (map[int]int, error) {
	rows, err := database.DB.Query("SELECT thread_id, vote_type FROM votes WHERE visitor_id = $1", visitorID)
	if err != nil { return nil, err }
	defer rows.Close()
	v := make(map[int]int)
	for rows.Next() {
		var tid, vt int
		rows.Scan(&tid, &vt)
		v[tid] = vt
	}
	return v, nil
}

func IsDeadMansSwitchActive() bool {
	var lastCheckinStr string
	err := database.DB.QueryRow("SELECT value FROM system_settings WHERE key = 'last_mod_checkin'").Scan(&lastCheckinStr)
	if err != nil { return false }
	lastCheckin, _ := time.Parse("2006-01-02 15:04:05", lastCheckinStr)
	return time.Since(lastCheckin) > 7*24*time.Hour
}

func GetThreadsByAuthor(authorName string) ([]Thread, error) {
	now := time.Now().UTC()
	rows, err := database.DB.Query(`
		SELECT t.id, t.title, t.content, t.author_name, t.category, t.parent_id, p.title as parent_title, t.parent_author, t.signal_score, t.is_locked, t.created_at, t.published_at 
		FROM threads t 
		LEFT JOIN threads p ON t.parent_id = p.id 
		WHERE t.author_name = $1 AND t.is_hidden = FALSE AND t.published_at <= $2 
		ORDER BY t.published_at DESC`, authorName, now)
	if err != nil { return nil, err }
	defer rows.Close()
	var threads []Thread
	for rows.Next() {
		var t Thread
		var pt, pa sql.NullString
		err := rows.Scan(&t.ID, &t.Title, &t.Content, &t.AuthorName, &t.Category, &t.ParentID, &pt, &pa, &t.SignalScore, &t.IsLocked, &t.CreatedAt, &t.PublishedAt)
		if err != nil { return nil, err }
		if pt.Valid { t.ParentTitle = pt.String }
		if pa.Valid { t.ParentAuthor = pa.String }
		threads = append(threads, t)
	}
	return threads, nil
}
