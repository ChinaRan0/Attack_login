package services

import (
	"batch-connector/internal/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const dbFileName = "connections.db"

// initDatabase 初始化数据库
func initDatabase() (*sql.DB, error) {
	dbPath := getDBPath()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Printf("数据库文件不存在，将创建新数据库: %s", dbPath)
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			log.Printf("创建数据库目录失败: %v", err)
		}
	}

	dsnPath := strings.ReplaceAll(dbPath, "\\", "/")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", dsnPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %v", err)
	}

	// 创建表
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("创建表失败: %v", err)
	}

	log.Printf("数据库初始化成功: %s", dbPath)
	return db, nil
}

// createTables 创建数据库表
func createTables(db *sql.DB) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS connections (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		ip TEXT NOT NULL,
		port TEXT NOT NULL,
		user TEXT,
		pass TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		message TEXT,
		result TEXT,
		logs TEXT,
		created_at TEXT NOT NULL,
		connected_at TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_type ON connections(type);
	CREATE INDEX IF NOT EXISTS idx_status ON connections(status);
	CREATE INDEX IF NOT EXISTS idx_created_at ON connections(created_at);
	`

	_, err := db.Exec(createTableSQL)
	return err
}

// connectionFromRow 从数据库行转换为 Connection 对象
func connectionFromRow(row *sql.Row) (*models.Connection, error) {
	var conn models.Connection
	var logsJSON string
	var createdAtStr, connectedAtStr string

	err := row.Scan(
		&conn.ID,
		&conn.Type,
		&conn.IP,
		&conn.Port,
		&conn.User,
		&conn.Pass,
		&conn.Status,
		&conn.Message,
		&conn.Result,
		&logsJSON,
		&createdAtStr,
		&connectedAtStr,
	)
	if err != nil {
		return nil, err
	}

	// 解析日志 JSON
	if logsJSON != "" {
		if err := json.Unmarshal([]byte(logsJSON), &conn.Logs); err != nil {
			conn.Logs = []string{}
		}
	} else {
		conn.Logs = []string{}
	}

	// 解析时间
	if createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			conn.CreatedAt = t
		}
	}
	if connectedAtStr != "" {
		if t, err := time.Parse(time.RFC3339, connectedAtStr); err == nil {
			conn.ConnectedAt = t
		}
	}

	return &conn, nil
}

// connectionFromRows 从 Rows 转换为 Connection 对象
func connectionFromRows(rows *sql.Rows) (*models.Connection, error) {
	var conn models.Connection
	var logsJSON string
	var createdAtStr, connectedAtStr string

	err := rows.Scan(
		&conn.ID,
		&conn.Type,
		&conn.IP,
		&conn.Port,
		&conn.User,
		&conn.Pass,
		&conn.Status,
		&conn.Message,
		&conn.Result,
		&logsJSON,
		&createdAtStr,
		&connectedAtStr,
	)
	if err != nil {
		return nil, err
	}

	// 解析日志 JSON
	if logsJSON != "" {
		if err := json.Unmarshal([]byte(logsJSON), &conn.Logs); err != nil {
			conn.Logs = []string{}
		}
	} else {
		conn.Logs = []string{}
	}

	// 解析时间
	if createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			conn.CreatedAt = t
		}
	}
	if connectedAtStr != "" {
		if t, err := time.Parse(time.RFC3339, connectedAtStr); err == nil {
			conn.ConnectedAt = t
		}
	}

	return &conn, nil
}

// connectionToValues 将 Connection 对象转换为数据库值
func connectionToValues(conn *models.Connection) ([]interface{}, error) {
	// 序列化日志
	logsJSON := "[]"
	if conn.Logs != nil && len(conn.Logs) > 0 {
		jsonData, err := json.Marshal(conn.Logs)
		if err != nil {
			return nil, err
		}
		logsJSON = string(jsonData)
	}

	// 格式化时间
	createdAtStr := conn.CreatedAt.Format(time.RFC3339)
	connectedAtStr := ""
	if !conn.ConnectedAt.IsZero() {
		connectedAtStr = conn.ConnectedAt.Format(time.RFC3339)
	}

	return []interface{}{
		conn.ID,
		conn.Type,
		conn.IP,
		conn.Port,
		conn.User,
		conn.Pass,
		conn.Status,
		conn.Message,
		conn.Result,
		logsJSON,
		createdAtStr,
		connectedAtStr,
	}, nil
}

// getDBPath 获取数据库文件路径
func getDBPath() string {
	// 尝试获取可执行文件所在目录
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		return filepath.Join(exeDir, dbFileName)
	}
	// 如果获取失败，使用当前工作目录
	return dbFileName
}
