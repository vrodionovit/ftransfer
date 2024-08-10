package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3" // Import for side-effects
)

type DownloadedFile struct {
	FileName     string
	FileSize     int64
	ServerName   string
	DownloadTime string
}

func openDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./downloads.db")
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}
	return db, nil
}

func createTable(db *sql.DB) error {
	createTableSQL := `CREATE TABLE IF NOT EXISTS downloaded_files (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"file_name" TEXT,
		"server_name" TEXT,
		"file_size" INTEGER,
		"download_time" TEXT
	);`
	_, err := db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("error creating table: %v", err)
	}
	return nil
}

func saveDownloadedFileEntry(db *sql.DB, file DownloadedFile) error {
	insertFileSQL := `INSERT INTO downloaded_files (file_name, file_size, download_time, server_name) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(insertFileSQL, file.FileName, file.FileSize, file.DownloadTime, file.ServerName)
	if err != nil {
		return fmt.Errorf("error inserting file entry: %v", err)
	}
	logger.Printf("File entry saved: %s, size: %s, downloaded at: %s\n", file.FileName, bytesToHumanReadable(file.FileSize), file.DownloadTime)
	return nil
}
func searchDownloadedFileEntries(db *sql.DB, fileName string, fileSize int64, serverName string) ([]DownloadedFile, error) {
	query := `SELECT file_name, file_size, download_time, server_name FROM downloaded_files WHERE file_name = ? AND file_size = ? AND server_name = ?`
	rows, err := db.Query(query, fileName, fileSize, serverName)
	if err != nil {
		return nil, fmt.Errorf("error querying file entries: %v", err)
	}
	defer rows.Close()

	var files []DownloadedFile
	for rows.Next() {
		var file DownloadedFile
		err := rows.Scan(&file.FileName, &file.FileSize, &file.DownloadTime, &file.ServerName)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}
		files = append(files, file)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return files, nil
}

func truncateDatabase(db *sql.DB) error {
	truncateSQL := `DELETE FROM downloaded_files`
	_, err := db.Exec(truncateSQL)
	if err != nil {
		return fmt.Errorf("error truncating table: %v", err)
	}
	logger.Println("All entries in the downloaded_files table have been deleted.")
	return nil
}

func deleteOldEntries(db *sql.DB) error {
	deleteSQL := `DELETE FROM downloaded_files WHERE download_time < datetime('now', '-7 days')`
	_, err := db.Exec(deleteSQL)
	if err != nil {
		return fmt.Errorf("error deleting old entries: %v", err)
	}
	logger.Println("Old entries older than 7 days have been deleted.")
	return nil
}
