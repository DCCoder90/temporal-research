//go:build !nogui

package main

import (
	"context"
	"database/sql"
	"fmt"
	"temporal-analyze/internal/analysis"
	"temporal-analyze/internal/config"
	"time"

	_ "modernc.org/sqlite"
)

// QueryResult is returned to the JS frontend by QueryDB.
type QueryResult struct {
	Columns   []string `json:"Columns"`
	Rows      [][]any  `json:"Rows"`
	RowCount  int      `json:"RowCount"`
	Truncated bool     `json:"Truncated"`
	SQLError  string   `json:"SQLError"` // non-empty on SQL syntax/runtime errors
}

const maxQueryRows = 10_000

// populateDB creates a new in-memory SQLite database populated with
// packets and grpc_calls tables from the analysis result.
func populateDB(result *analysis.Result) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&mode=memory")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory DB: %w", err)
	}

	// Use a single connection to ensure the in-memory DB persists.
	db.SetMaxOpenConns(1)

	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := insertPackets(db, result); err != nil {
		db.Close()
		return nil, err
	}
	if err := insertGRPCCalls(db, result); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE packets (
			time        REAL,
			src_ip      TEXT,
			dst_ip      TEXT,
			src         TEXT,
			dst         TEXT,
			src_port    TEXT,
			dst_port    TEXT,
			protocol    TEXT,
			bytes       INTEGER,
			tcp_stream  INTEGER,
			tcp_len     INTEGER,
			tcp_flags   INTEGER,
			retransmit  INTEGER,
			rtt         REAL
		);
		CREATE TABLE grpc_calls (
			time        REAL,
			src         TEXT,
			dst         TEXT,
			full_path   TEXT,
			service     TEXT,
			method      TEXT,
			tcp_stream  INTEGER,
			stream_id   INTEGER,
			status_code INTEGER
		);
	`)
	return err
}

func insertPackets(db *sql.DB, result *analysis.Result) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO packets VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, p := range result.Packets {
		retransmit := 0
		if p.Retransmit {
			retransmit = 1
		}
		_, err = stmt.Exec(
			p.T,
			p.Src,
			p.Dst,
			config.Resolve(p.Src),
			config.Resolve(p.Dst),
			p.Sport,
			p.Dport,
			p.Proto,
			p.Len,
			p.TCPStream,
			p.TCPLen,
			p.TCPFlags,
			retransmit,
			p.RTT,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func insertGRPCCalls(db *sql.DB, result *analysis.Result) error {
	if len(result.GRPCCalls) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO grpc_calls VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, c := range result.GRPCCalls {
		_, err = stmt.Exec(c.T, c.Src, c.Dst, c.FullPath, c.Service, c.Method, c.TCPStream, c.StreamID, c.StatusCode)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// QueryDB executes a SQL query against the in-memory packet database
// populated by the last Analyze call. SQL errors are returned in QueryResult.SQLError
// rather than as Go errors so the frontend can display them inline.
func (a *App) QueryDB(query string) (*QueryResult, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no analysis loaded — run Analyze first")
	}

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return &QueryResult{SQLError: err.Error()}, nil
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return &QueryResult{SQLError: err.Error()}, nil
	}

	result := &QueryResult{Columns: cols}
	scanBuf := make([]any, len(cols))
	scanPtrs := make([]any, len(cols))
	for i := range scanBuf {
		scanPtrs[i] = &scanBuf[i]
	}

	for rows.Next() {
		if result.RowCount >= maxQueryRows {
			result.Truncated = true
			break
		}
		if err := rows.Scan(scanPtrs...); err != nil {
			return &QueryResult{SQLError: err.Error()}, nil
		}
		row := make([]any, len(cols))
		copy(row, scanBuf)
		result.Rows = append(result.Rows, row)
		result.RowCount++
	}
	if err := rows.Err(); err != nil {
		return &QueryResult{SQLError: err.Error()}, nil
	}

	return result, nil
}
