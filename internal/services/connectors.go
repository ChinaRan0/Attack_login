package services

import (
	"batch-connector/internal/models"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	go_ora "github.com/sijms/go-ora/v2"
	"github.com/hirochachacha/go-smb2"
	"github.com/jlaffaye/ftp"
	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// addLog 添加日志
func (s *ConnectorService) addLog(conn *models.Connection, message string) {
	if conn.Logs == nil {
		conn.Logs = []string{}
	}
	timestamp := time.Now().Format("15:04:05")
	logMsg := fmt.Sprintf("[%s] %s", timestamp, message)
	conn.Logs = append(conn.Logs, logMsg)
	log.Printf("[%s %s:%s] %s", conn.Type, conn.IP, conn.Port, logMsg)
}

// Connect 执行连接测试
func (s *ConnectorService) Connect(conn *models.Connection) {
	conn.Status = "pending"
	conn.Message = "连接中..."
	conn.Logs = []string{}

	// 更新数据库状态
	s.UpdateConnection(conn)

	s.addLog(conn, fmt.Sprintf("开始连接 %s 服务", conn.Type))
	s.addLog(conn, fmt.Sprintf("目标地址: %s:%s", conn.IP, conn.Port))
	if conn.User != "" {
		s.addLog(conn, fmt.Sprintf("用户名: %s", conn.User))
	}
	if conn.Pass != "" {
		s.addLog(conn, "使用密码认证")
	} else {
		s.addLog(conn, "尝试未授权访问或无密码连接")
	}

	switch strings.ToLower(conn.Type) {
	case "redis":
		s.connectRedis(conn)
	case "ftp":
		s.connectFTP(conn)
	case "postgresql", "postgres":
		s.connectPostgreSQL(conn)
	case "mysql":
		s.connectMySQL(conn)
	case "rabbitmq":
		s.connectRabbitMQ(conn)
	case "sqlserver", "mssql", "sql":
		s.connectSQLServer(conn)
	case "ssh":
		s.connectSSH(conn)
	case "mongodb", "mongo":
		s.connectMongoDB(conn)
	case "smb", "samba", "cifs":
		s.connectSMB(conn)
	case "wmi":
		s.connectWMI(conn)
	case "mqtt":
		s.connectMQTT(conn)
	case "oracle":
		s.connectOracle(conn)
	default:
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("不支持的服务类型: %s", conn.Type)
		s.addLog(conn, fmt.Sprintf("错误: 不支持的服务类型 %s", conn.Type))
	}

	// 连接完成后更新数据库
	s.UpdateConnection(conn)
}

// connectRedis 连接 Redis
func (s *ConnectorService) connectRedis(conn *models.Connection) {
	ctx := context.Background()
	addr := net.JoinHostPort(conn.IP, conn.Port)
	s.addLog(conn, fmt.Sprintf("连接地址: %s", addr))
	
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
	}

	// 创建 Dialer
	var dialer func(ctx context.Context, network, addr string) (net.Conn, error)
	if s.config.Proxy.Enabled {
		proxyDialer, err := s.getProxyDialer()
		if err != nil {
			s.addLog(conn, fmt.Sprintf("✗ 创建代理 Dialer 失败: %v", err))
			conn.Status = "failed"
			conn.Message = fmt.Sprintf("代理配置错误: %v", err)
			return
		}
		if contextDialer, ok := proxyDialer.(proxy.ContextDialer); ok {
			dialer = contextDialer.DialContext
		} else {
			dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return proxyDialer.Dial(network, addr)
			}
		}
	}

	// 如果用户提供了密码，直接使用密码连接，跳过未授权访问
	if conn.Pass != "" {
		s.addLog(conn, "尝试使用密码连接")
		opts := &redis.Options{
			Addr:     addr,
			Password: conn.Pass,
			DB:       0,
		}
		if dialer != nil {
			opts.Dialer = dialer
		}
		rdb := redis.NewClient(opts)
		_, err := rdb.Ping(ctx).Result()
		if err == nil {
			s.addLog(conn, "✓ 密码认证成功")
			s.addLog(conn, "获取数据库信息")
			result := s.getRedisDatabases(addr, conn.Pass, ctx)
			conn.Status = "success"
			conn.Message = "连接成功（使用密码）"
			conn.Result = result
			conn.ConnectedAt = time.Now()
			rdb.Close()
			return
		}
		s.addLog(conn, fmt.Sprintf("✗ 密码认证失败: %v", err))
		rdb.Close()
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("连接失败: %v", err)
		s.addLog(conn, "密码认证失败")
		return
	}

	// 如果没有提供密码，尝试未授权访问
	s.addLog(conn, "尝试未授权访问（无密码）")
	opts := &redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	}
	if dialer != nil {
		opts.Dialer = dialer
	}
	rdb := redis.NewClient(opts)

	_, err := rdb.Ping(ctx).Result()
	if err == nil {
		s.addLog(conn, "✓ 未授权访问成功")
		s.addLog(conn, "获取数据库信息")
		result := s.getRedisDatabases(addr, "", ctx)
		conn.Status = "success"
		conn.Message = "连接成功（未授权访问）"
		conn.Result = result
		conn.ConnectedAt = time.Now()
		rdb.Close()
		return
	}
	s.addLog(conn, fmt.Sprintf("✗ 未授权访问失败: %v", err))
	rdb.Close()

	conn.Status = "failed"
	conn.Message = fmt.Sprintf("连接失败: %v", err)
	s.addLog(conn, "所有连接尝试均失败")
}

// getRedisDatabases 获取 Redis 数据库信息
func (s *ConnectorService) getRedisDatabases(addr, password string, ctx context.Context) string {
	var results []string

	// 创建临时客户端用于获取信息
	tempRdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	defer tempRdb.Close()

	// 获取 keyspace 信息（显示有数据的数据库）
	info, err := tempRdb.Info(ctx, "keyspace").Result()
	if err == nil && info != "" {
		results = append(results, fmt.Sprintf("Keyspace 信息:\n%s", info))
	} else {
		results = append(results, fmt.Sprintf("获取 Keyspace 信息失败: %v", err))
	}

	// 获取配置的数据库数量
	config, err := tempRdb.ConfigGet(ctx, "databases").Result()
	if err == nil && len(config) > 0 {
		results = append(results, fmt.Sprintf("配置的数据库数量: %s", config[1]))
	}

	// 尝试检查每个数据库（0-15）是否有数据
	var databasesWithData []string
	for i := 0; i < 16; i++ {
		testRdb := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       i,
		})
		keys, err := testRdb.DBSize(ctx).Result()
		testRdb.Close()
		if err == nil && keys > 0 {
			databasesWithData = append(databasesWithData, fmt.Sprintf("db%d (%d keys)", i, keys))
		}
	}

	if len(databasesWithData) > 0 {
		results = append(results, fmt.Sprintf("有数据的数据库: %s", strings.Join(databasesWithData, ", ")))
	} else {
		results = append(results, "未发现包含数据的数据库")
	}

	return strings.Join(results, "\n")
}

// connectFTP 连接 FTP
func (s *ConnectorService) connectFTP(conn *models.Connection) {
	addr := net.JoinHostPort(conn.IP, conn.Port)
	s.addLog(conn, fmt.Sprintf("连接地址: %s", addr))
	
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
		s.addLog(conn, "注意: FTP 连接暂不支持代理，将尝试直接连接")
	}

	var ftpConn *ftp.ServerConn
	var err error
	var connected bool
	var loginType string

	// 如果用户提供了用户名和密码，直接使用，跳过匿名登录
	if conn.User != "" && conn.Pass != "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
		ftpConn, err = ftp.Dial(addr, ftp.DialWithTimeout(5*time.Second))
		if err == nil {
			err = ftpConn.Login(conn.User, conn.Pass)
			if err == nil {
				s.addLog(conn, "✓ 密码认证成功")
				connected = true
				loginType = "使用用户名密码"
			} else {
				s.addLog(conn, fmt.Sprintf("✗ 密码认证失败: %v", err))
				ftpConn.Quit()
			}
		} else {
			s.addLog(conn, fmt.Sprintf("✗ FTP 连接失败: %v", err))
		}
		if !connected {
			conn.Status = "failed"
			conn.Message = fmt.Sprintf("连接失败: %v", err)
			s.addLog(conn, "密码认证失败")
			return
		}
	} else {
		// 尝试匿名登录
		s.addLog(conn, "尝试匿名登录（anonymous/anonymous）")
		ftpConn, err = ftp.Dial(addr, ftp.DialWithTimeout(5*time.Second))
		if err == nil {
			err = ftpConn.Login("anonymous", "anonymous")
			if err == nil {
				s.addLog(conn, "✓ 匿名登录成功")
				connected = true
				loginType = "匿名登录"
			} else {
				s.addLog(conn, fmt.Sprintf("✗ 匿名登录失败: %v", err))
				ftpConn.Quit()
			}
		} else {
			s.addLog(conn, fmt.Sprintf("✗ FTP 连接失败: %v", err))
		}

		// 尝试未授权访问（无密码）
		if !connected && conn.User != "" && conn.Pass == "" {
			s.addLog(conn, fmt.Sprintf("尝试用户 %s 无密码登录", conn.User))
			ftpConn, err = ftp.Dial(addr, ftp.DialWithTimeout(5*time.Second))
			if err == nil {
				err = ftpConn.Login(conn.User, "")
				if err == nil {
					s.addLog(conn, "✓ 无密码登录成功")
					connected = true
					loginType = "无密码"
				} else {
					s.addLog(conn, fmt.Sprintf("✗ 无密码登录失败: %v", err))
					ftpConn.Quit()
				}
			}
		}
	}

	if !connected {
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("连接失败: %v", err)
		s.addLog(conn, "所有连接尝试均失败")
		return
	}

	// 连接成功，执行 dir 命令
	s.addLog(conn, "执行 dir 命令")
	result := s.getFTPDirectoryList(ftpConn)
	conn.Status = "success"
	conn.Message = fmt.Sprintf("连接成功（%s）", loginType)
	conn.Result = result
	conn.ConnectedAt = time.Now()
	ftpConn.Quit()
}

// getFTPDirectoryList 获取 FTP 目录列表（相当于 dir 命令）
func (s *ConnectorService) getFTPDirectoryList(ftpConn *ftp.ServerConn) string {
	var results []string
	results = append(results, "目录列表:")
	results = append(results, strings.Repeat("-", 80))

	// 获取当前工作目录
	pwd, err := ftpConn.CurrentDir()
	if err == nil {
		results = append(results, fmt.Sprintf("当前目录: %s", pwd))
		results = append(results, "")
	}

	// 执行 LIST 命令获取目录列表
	entries, err := ftpConn.List(".")
	if err != nil {
		results = append(results, fmt.Sprintf("获取目录列表失败: %v", err))
		return strings.Join(results, "\n")
	}

	if len(entries) == 0 {
		results = append(results, "当前目录为空")
	} else {
		results = append(results, fmt.Sprintf("共找到 %d 个项目:", len(entries)))
		results = append(results, "")
		results = append(results, fmt.Sprintf("%-10s %-15s %-20s %-30s", "类型", "大小", "修改时间", "名称"))
		results = append(results, strings.Repeat("-", 80))

		for _, entry := range entries {
			fileType := "文件"
			if entry.Type == ftp.EntryTypeFolder {
				fileType = "目录"
			}

			size := fmt.Sprintf("%d", entry.Size)
			if entry.Size == 0 && entry.Type == ftp.EntryTypeFolder {
				size = "-"
			}

			timeStr := entry.Time.Format("2006-01-02 15:04:05")
			if entry.Time.IsZero() {
				timeStr = "-"
			}

			results = append(results, fmt.Sprintf("%-10s %-15s %-20s %-30s", fileType, size, timeStr, entry.Name))
		}
	}

	return strings.Join(results, "\n")
}

// connectPostgreSQL 连接 PostgreSQL
func (s *ConnectorService) connectPostgreSQL(conn *models.Connection) {
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
		s.addLog(conn, "注意: PostgreSQL 连接暂不支持代理，将尝试直接连接")
	}
	
	// 如果用户提供了用户名和密码，直接使用，跳过默认用户连接
	if conn.User != "" && conn.Pass != "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable connect_timeout=5",
			conn.IP, conn.Port, conn.User, conn.Pass)
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				s.addLog(conn, "✓ 密码认证成功")
				s.addLog(conn, "执行查询: SELECT * FROM pg_database")
				result := s.getPostgreSQLDatabases(db)
				conn.Status = "success"
				conn.Message = "连接成功（使用用户名密码）"
				conn.Result = result
				conn.ConnectedAt = time.Now()
				db.Close()
				return
			}
			s.addLog(conn, fmt.Sprintf("✗ 用户认证失败: %v", err))
			db.Close()
		} else {
			s.addLog(conn, fmt.Sprintf("✗ 数据库连接失败: %v", err))
		}
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("连接失败: %v", err)
		s.addLog(conn, "密码认证失败")
		return
	}

	// 尝试未授权访问（使用默认用户 postgres，无密码）
	s.addLog(conn, "尝试默认用户 postgres 无密码连接")
	dsn := fmt.Sprintf("host=%s port=%s user=postgres password= dbname=postgres sslmode=disable connect_timeout=5",
		conn.IP, conn.Port)
	db, err := sql.Open("postgres", dsn)
	if err == nil {
		err = db.Ping()
		if err == nil {
			s.addLog(conn, "✓ 默认用户 postgres 无密码连接成功")
			s.addLog(conn, "执行查询: SELECT * FROM pg_database")
			result := s.getPostgreSQLDatabases(db)
			conn.Status = "success"
			conn.Message = "连接成功（未授权访问，默认用户 postgres）"
			conn.Result = result
			conn.ConnectedAt = time.Now()
			db.Close()
			return
		}
		s.addLog(conn, fmt.Sprintf("✗ 默认用户连接失败: %v", err))
		db.Close()
	} else {
		s.addLog(conn, fmt.Sprintf("✗ 数据库连接失败: %v", err))
	}

	// 尝试使用提供的用户名（无密码）
	if conn.User != "" {
		password := conn.Pass
		if password == "" {
			s.addLog(conn, fmt.Sprintf("尝试用户 %s 无密码连接", conn.User))
		} else {
			s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
		}
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable connect_timeout=5",
			conn.IP, conn.Port, conn.User, password)
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				if password == "" {
					s.addLog(conn, "✓ 无密码连接成功")
					conn.Status = "success"
					conn.Message = "连接成功（无密码）"
				} else {
					s.addLog(conn, "✓ 密码认证成功")
					conn.Status = "success"
					conn.Message = "连接成功（使用用户名密码）"
				}
				s.addLog(conn, "执行查询: SELECT * FROM pg_database")
				result := s.getPostgreSQLDatabases(db)
				conn.Result = result
				conn.ConnectedAt = time.Now()
				db.Close()
				return
			}
			s.addLog(conn, fmt.Sprintf("✗ 用户认证失败: %v", err))
			db.Close()
		}
	}

	conn.Status = "failed"
	conn.Message = fmt.Sprintf("连接失败: %v", err)
	s.addLog(conn, "所有连接尝试均失败")
}

// getPostgreSQLDatabases 获取 PostgreSQL 数据库列表
func (s *ConnectorService) getPostgreSQLDatabases(db *sql.DB) string {
	query := "SELECT datname, pg_size_pretty(pg_database_size(datname)) as size, datcollate, datctype FROM pg_database ORDER BY datname"
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Sprintf("查询失败: %v", err)
	}
	defer rows.Close()

	var results []string
	results = append(results, "数据库列表:")
	results = append(results, fmt.Sprintf("%-20s %-15s %-15s %-15s", "数据库名", "大小", "排序规则", "字符集"))
	results = append(results, strings.Repeat("-", 65))

	for rows.Next() {
		var datname, size, datcollate, datctype string
		if err := rows.Scan(&datname, &size, &datcollate, &datctype); err != nil {
			results = append(results, fmt.Sprintf("读取行失败: %v", err))
			continue
		}
		results = append(results, fmt.Sprintf("%-20s %-15s %-15s %-15s", datname, size, datcollate, datctype))
	}

	if err := rows.Err(); err != nil {
		results = append(results, fmt.Sprintf("遍历行时出错: %v", err))
	}

	return strings.Join(results, "\n")
}

// connectMySQL 连接 MySQL
func (s *ConnectorService) connectMySQL(conn *models.Connection) {
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
		s.addLog(conn, "注意: MySQL 连接暂不支持代理，将尝试直接连接")
	}
	
	// 如果提供了密码，直接使用密码认证
	if conn.Pass != "" && conn.User != "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/mysql?timeout=5s",
			conn.User, conn.Pass, conn.IP, conn.Port)
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				s.addLog(conn, "✓ 密码认证成功")
				s.addLog(conn, "执行查询: SHOW DATABASES")
				result := s.getMySQLDatabases(db)
				conn.Status = "success"
				conn.Message = "连接成功（使用用户名密码）"
				conn.Result = result
				conn.ConnectedAt = time.Now()
				db.Close()
				return
			}
			s.addLog(conn, fmt.Sprintf("✗ 密码认证失败: %v", err))
			db.Close()
		} else {
			s.addLog(conn, fmt.Sprintf("✗ 数据库连接失败: %v", err))
		}
		// 密码认证失败，不再尝试其他方式
		conn.Status = "failed"
		conn.Message = "连接失败: 密码认证失败"
		s.addLog(conn, "密码认证失败，不再尝试无密码连接")
		return
	}

	// 如果没有提供密码，尝试未授权访问（root 无密码）
	s.addLog(conn, "尝试 root 用户无密码连接")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/mysql?timeout=5s",
		"root", "", conn.IP, conn.Port)
	db, err := sql.Open("mysql", dsn)
	if err == nil {
		err = db.Ping()
		if err == nil {
			s.addLog(conn, "✓ root 用户无密码连接成功")
			s.addLog(conn, "执行查询: SHOW DATABASES")
			result := s.getMySQLDatabases(db)
			conn.Status = "success"
			conn.Message = "连接成功（未授权访问，root 无密码）"
			conn.Result = result
			conn.ConnectedAt = time.Now()
			db.Close()
			return
		}
		s.addLog(conn, fmt.Sprintf("✗ root 用户连接失败: %v", err))
		db.Close()
	} else {
		s.addLog(conn, fmt.Sprintf("✗ 数据库连接失败: %v", err))
	}

	// 尝试使用提供的用户名（无密码）
	if conn.User != "" && conn.Pass == "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 无密码连接", conn.User))
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/mysql?timeout=5s",
			conn.User, "", conn.IP, conn.Port)
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				s.addLog(conn, "✓ 无密码连接成功")
				s.addLog(conn, "执行查询: SHOW DATABASES")
				result := s.getMySQLDatabases(db)
				conn.Status = "success"
				conn.Message = "连接成功（无密码）"
				conn.Result = result
				conn.ConnectedAt = time.Now()
				db.Close()
				return
			}
			s.addLog(conn, fmt.Sprintf("✗ 用户认证失败: %v", err))
			db.Close()
		}
	}

	conn.Status = "failed"
	conn.Message = fmt.Sprintf("连接失败: %v", err)
	s.addLog(conn, "所有连接尝试均失败")
}

// connectSQLServer 连接 SQL Server
func (s *ConnectorService) connectSQLServer(conn *models.Connection) {
	port := conn.Port
	if port == "" {
		port = "1433"
	}
	server := net.JoinHostPort(conn.IP, port)
	s.addLog(conn, fmt.Sprintf("目标 SQL Server 地址: %s", server))
	
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
		s.addLog(conn, "注意: SQL Server 连接暂不支持代理，将尝试直接连接")
	}

	type attempt struct {
		user  string
		pass  string
		label string
	}

	var attempts []attempt
	seen := make(map[string]struct{})
	addAttempt := func(user, pass, label string) {
		key := fmt.Sprintf("%s|%s", user, pass)
		if _, exists := seen[key]; exists {
			return
		}
		attempts = append(attempts, attempt{
			user:  user,
			pass:  pass,
			label: label,
		})
		seen[key] = struct{}{}
	}

	// 优先尝试用户提供的凭据
	if conn.User != "" || conn.Pass != "" {
		addAttempt(conn.User, conn.Pass, "用户提供的凭据")
		// 如果用户只提供了用户名，补充一次无密码尝试
		if conn.Pass == "" && conn.User != "" {
			addAttempt(conn.User, "", "用户提供的用户名（无密码）")
		}
	}

	// 追加常见的弱口令/默认凭据
	defaultAttempts := []attempt{
		{user: "sa", pass: "", label: "默认用户 sa 无密码"},
		{user: "sa", pass: "sa", label: "常见弱口令 sa/sa"},
		{user: "sa", pass: "123456", label: "常见弱口令 sa/123456"},
		{user: "sa", pass: "P@ssw0rd", label: "常见弱口令 sa/P@ssw0rd"},
		{user: "sa", pass: "Password123", label: "常见弱口令 sa/Password123"},
		{user: "", pass: "", label: "无凭据"},
	}
	for _, att := range defaultAttempts {
		addAttempt(att.user, att.pass, att.label)
	}

	// 兜底，至少要有一个尝试
	if len(attempts) == 0 {
		addAttempt("sa", "", "默认用户 sa 无密码")
		addAttempt("", "", "无凭据")
	}

	var lastErr error
	for _, att := range attempts {
		if att.user != "" {
			if att.pass == "" {
				s.addLog(conn, fmt.Sprintf("尝试 SQL Server 用户 %s 无密码连接（%s）", att.user, att.label))
			} else {
				s.addLog(conn, fmt.Sprintf("尝试 SQL Server 用户 %s 密码认证（%s）", att.user, att.label))
			}
		} else {
			s.addLog(conn, fmt.Sprintf("尝试 SQL Server 无凭据连接（%s）", att.label))
		}

		dsn := buildSQLServerDSN(server, att.user, att.pass)
		db, err := sql.Open("sqlserver", dsn)
		if err != nil {
			s.addLog(conn, fmt.Sprintf("✗ 创建 SQL Server 连接失败: %v", err))
			lastErr = err
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err != nil {
			s.addLog(conn, fmt.Sprintf("✗ SQL Server 认证失败: %v", err))
			lastErr = err
			db.Close()
			continue
		}

		s.addLog(conn, "✓ SQL Server 连接成功")
		s.addLog(conn, "执行查询: SELECT name AS DatabaseName FROM sys.databases")

		result := s.getSQLServerDatabases(db)
		conn.Status = "success"
		if att.user != "" {
			if att.pass == "" {
				conn.Message = fmt.Sprintf("连接成功（SQL Server 用户 %s 无密码）", att.user)
			} else {
				conn.Message = fmt.Sprintf("连接成功（SQL Server 用户 %s）", att.user)
			}
		} else {
			conn.Message = "连接成功（SQL Server 无凭据）"
		}
		conn.Result = result
		conn.ConnectedAt = time.Now()
		db.Close()
		return
	}

	conn.Status = "failed"
	failMsg := "连接失败: 所有 SQL Server 尝试均失败"
	if lastErr != nil {
		failMsg = fmt.Sprintf("%s（最后错误: %v）", failMsg, lastErr)
	}
	conn.Message = failMsg
	s.addLog(conn, "所有 SQL Server 连接尝试均失败")
	if lastErr != nil {
		s.addLog(conn, fmt.Sprintf("最后错误: %v", lastErr))
	}
}

// buildSQLServerDSN 构建 SQL Server DSN
func buildSQLServerDSN(server, user, pass string) string {
	const commonParams = "?encrypt=disable"
	if user == "" {
		return fmt.Sprintf("sqlserver://%s%s", server, commonParams)
	}

	escapedUser := url.QueryEscape(user)
	if pass == "" {
		return fmt.Sprintf("sqlserver://%s@%s%s", escapedUser, server, commonParams)
	}
	escapedPass := url.QueryEscape(pass)
	return fmt.Sprintf("sqlserver://%s:%s@%s%s", escapedUser, escapedPass, server, commonParams)
}

// getSQLServerDatabases 获取 SQL Server 数据库列表
func (s *ConnectorService) getSQLServerDatabases(db *sql.DB) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, "SELECT name AS DatabaseName FROM sys.databases")
	if err != nil {
		return fmt.Sprintf("查询失败: %v", err)
	}
	defer rows.Close()

	var results []string
	results = append(results, "数据库列表:")
	results = append(results, strings.Repeat("-", 40))

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			results = append(results, fmt.Sprintf("读取行失败: %v", err))
			continue
		}
		results = append(results, fmt.Sprintf("- %s", name))
	}

	if err := rows.Err(); err != nil {
		results = append(results, fmt.Sprintf("遍历行时出错: %v", err))
	}

	if len(results) == 2 {
		results = append(results, "未获取到任何数据库")
	}

	return strings.Join(results, "\n")
}

// getMySQLDatabases 获取 MySQL 数据库列表
func (s *ConnectorService) getMySQLDatabases(db *sql.DB) string {
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return fmt.Sprintf("查询失败: %v", err)
	}
	defer rows.Close()

	var results []string
	results = append(results, "数据库列表:")
	results = append(results, "数据库名")
	results = append(results, strings.Repeat("-", 30))

	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			results = append(results, fmt.Sprintf("读取行失败: %v", err))
			continue
		}
		results = append(results, database)
	}

	if err := rows.Err(); err != nil {
		results = append(results, fmt.Sprintf("遍历行时出错: %v", err))
	}

	return strings.Join(results, "\n")
}

// connectRabbitMQ 连接 RabbitMQ
func (s *ConnectorService) connectRabbitMQ(conn *models.Connection) {
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
		s.addLog(conn, "注意: RabbitMQ 连接暂不支持代理，将尝试直接连接")
	}
	
	var username, password string
	var connected bool

	// 如果用户提供了用户名和密码，直接使用，跳过默认用户连接
	if conn.User != "" && conn.Pass != "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
		amqpURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", conn.User, conn.Pass, conn.IP, conn.Port)
		client, err := amqp.Dial(amqpURL)
		if err == nil {
			s.addLog(conn, "✓ 密码认证成功")
			username = conn.User
			password = conn.Pass
			connected = true
			client.Close()
		} else {
			s.addLog(conn, fmt.Sprintf("✗ 用户认证失败: %v", err))
			conn.Status = "failed"
			conn.Message = fmt.Sprintf("连接失败: %v", err)
			s.addLog(conn, "密码认证失败")
			return
		}
	} else {
		// 尝试未授权访问（默认用户 guest/guest）
		s.addLog(conn, "尝试默认用户 guest/guest 连接")
		amqpURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", conn.IP, conn.Port)
		client, err := amqp.Dial(amqpURL)
		if err == nil {
			s.addLog(conn, "✓ 默认用户 guest/guest 连接成功")
			username = "guest"
			password = "guest"
			connected = true
			client.Close()
		} else {
			s.addLog(conn, fmt.Sprintf("✗ 默认用户连接失败: %v", err))

			// 尝试使用提供的用户名（无密码）
			if conn.User != "" {
				pass := conn.Pass
				if pass == "" {
					s.addLog(conn, fmt.Sprintf("尝试用户 %s 无密码连接", conn.User))
				} else {
					s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
				}
				amqpURL = fmt.Sprintf("amqp://%s:%s@%s:%s/", conn.User, pass, conn.IP, conn.Port)
				client, err = amqp.Dial(amqpURL)
				if err == nil {
					if pass == "" {
						s.addLog(conn, "✓ 无密码连接成功")
					} else {
						s.addLog(conn, "✓ 密码认证成功")
					}
					username = conn.User
					password = pass
					connected = true
					client.Close()
				} else {
					s.addLog(conn, fmt.Sprintf("✗ 用户认证失败: %v", err))
				}
			}
		}
	}

	if !connected {
		conn.Status = "failed"
		conn.Message = "连接失败: 所有尝试均失败"
		s.addLog(conn, "所有连接尝试均失败")
		return
	}

	// 连接成功，执行 list_connections
	s.addLog(conn, "执行 list_connections")
	result := s.getRabbitMQConnections(conn.IP, username, password)
	conn.Status = "success"
	if conn.User != "" && conn.Pass != "" {
		conn.Message = "连接成功（使用用户名密码）"
	} else if username == "guest" {
		conn.Message = "连接成功（未授权访问，默认用户 guest/guest）"
	} else {
		conn.Message = "连接成功"
	}
	conn.Result = result
	conn.ConnectedAt = time.Now()
}

// getRabbitMQConnections 获取 RabbitMQ 连接列表
func (s *ConnectorService) getRabbitMQConnections(ip, username, password string) string {
	// RabbitMQ Management API 常见端口
	managementPorts := []string{"15672", "15671", "15673"}

	var results []string
	results = append(results, "连接列表:")
	results = append(results, strings.Repeat("-", 80))

	for _, port := range managementPorts {
		url := fmt.Sprintf("http://%s:%s/api/connections", ip, port)
		s.addLogForConnections(&results, fmt.Sprintf("尝试连接 Management API (端口 %s)", port))

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			s.addLogForConnections(&results, fmt.Sprintf("创建请求失败: %v", err))
			continue
		}

		// 设置 Basic Auth
		req.SetBasicAuth(username, password)

		// 创建 HTTP 客户端，设置超时
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		resp, err := client.Do(req)
		if err != nil {
			s.addLogForConnections(&results, fmt.Sprintf("请求失败: %v", err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			s.addLogForConnections(&results, fmt.Sprintf("HTTP 状态码: %d", resp.StatusCode))
			continue
		}

		// 解析 JSON 响应
		var connections []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&connections); err != nil {
			s.addLogForConnections(&results, fmt.Sprintf("解析 JSON 失败: %v", err))
			continue
		}

		s.addLogForConnections(&results, fmt.Sprintf("✓ 成功获取连接列表 (端口 %s, 共 %d 个连接)", port, len(connections)))
		results = append(results, "")

		if len(connections) == 0 {
			results = append(results, "当前没有活跃连接")
		} else {
			// 格式化输出连接信息
			for i, conn := range connections {
				results = append(results, fmt.Sprintf("连接 #%d:", i+1))
				if name, ok := conn["name"].(string); ok {
					results = append(results, fmt.Sprintf("  名称: %s", name))
				}
				if user, ok := conn["user"].(string); ok {
					results = append(results, fmt.Sprintf("  用户: %s", user))
				}
				if peerHost, ok := conn["peer_host"].(string); ok {
					results = append(results, fmt.Sprintf("  对端地址: %s", peerHost))
				}
				if peerPort, ok := conn["peer_port"].(float64); ok {
					results = append(results, fmt.Sprintf("  对端端口: %.0f", peerPort))
				}
				if state, ok := conn["state"].(string); ok {
					results = append(results, fmt.Sprintf("  状态: %s", state))
				}
				if channels, ok := conn["channels"].(float64); ok {
					results = append(results, fmt.Sprintf("  通道数: %.0f", channels))
				}
				if connectedAt, ok := conn["connected_at"].(float64); ok {
					connectedTime := time.Unix(int64(connectedAt)/1000, 0)
					results = append(results, fmt.Sprintf("  连接时间: %s", connectedTime.Format("2006-01-02 15:04:05")))
				}
				results = append(results, "")
			}
		}

		// 成功获取后不再尝试其他端口
		return strings.Join(results, "\n")
	}

	results = append(results, "无法连接到 Management API（已尝试端口: 15672, 15671, 15673）")
	return strings.Join(results, "\n")
}

// addLogForConnections 为连接列表添加日志（辅助函数）
func (s *ConnectorService) addLogForConnections(results *[]string, message string) {
	*results = append(*results, message)
}

// connectSSH 连接 SSH
func (s *ConnectorService) connectSSH(conn *models.Connection) {
	addr := net.JoinHostPort(conn.IP, conn.Port)
	s.addLog(conn, fmt.Sprintf("连接地址: %s", addr))
	
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
	}

	// 如果用户名为空，尝试常见默认用户名
	users := []string{conn.User}
	if conn.User == "" {
		users = []string{"root", "admin", "ubuntu", "centos"}
		s.addLog(conn, "用户名为空，将尝试常见默认用户名")
	}

	// 如果提供了密码，只尝试密码认证
	if conn.Pass != "" {
		s.addLog(conn, fmt.Sprintf("使用提供的密码进行认证（密码长度: %d）", len(conn.Pass)))
		for _, user := range users {
			if user == "" {
				continue
			}
			s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", user))
			config := &ssh.ClientConfig{
				User:            user,
				Auth:            []ssh.AuthMethod{ssh.Password(conn.Pass)},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				Timeout:         5 * time.Second,
			}

			// 使用代理或直接连接
			var client *ssh.Client
			var err error
			if s.config.Proxy.Enabled {
				proxyDialer, dialErr := s.getProxyDialer()
				if dialErr != nil {
					s.addLog(conn, fmt.Sprintf("✗ 创建代理 Dialer 失败: %v", dialErr))
					continue
				}
				connProxy, dialErr := proxyDialer.Dial("tcp", addr)
				if dialErr != nil {
					s.addLog(conn, fmt.Sprintf("✗ 通过代理连接失败: %v", dialErr))
					continue
				}
				sshConn, chans, reqs, err := ssh.NewClientConn(connProxy, addr, config)
				if err == nil {
					client = ssh.NewClient(sshConn, chans, reqs)
				}
			} else {
				client, err = ssh.Dial("tcp", addr, config)
			}
			if err == nil {
				s.addLog(conn, fmt.Sprintf("✓ 用户 %s 密码认证成功", user))
				s.addLog(conn, "执行命令: whoami, ip addr")
				// 执行命令
				result := s.executeSSHCommands(client)
				conn.Status = "success"
				conn.Message = fmt.Sprintf("连接成功（用户: %s）", user)
				conn.Result = result
				conn.ConnectedAt = time.Now()
				client.Close()
				return
			}
			s.addLog(conn, fmt.Sprintf("✗ 用户 %s 密码认证失败: %v", user, err))
		}
		// 如果提供了密码但所有尝试都失败，不再尝试密钥认证
		conn.Status = "failed"
		conn.Message = "连接失败: 密码认证失败"
		s.addLog(conn, "密码认证失败，不再尝试密钥认证")
		return
	}

	// 如果没有提供密码，尝试密钥认证或无密码连接
	s.addLog(conn, "未提供密码，尝试密钥认证或无密码连接")
	for _, user := range users {
		if user == "" {
			continue
		}
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密钥认证", user))
		config := &ssh.ClientConfig{
			User:            user,
			Auth:            []ssh.AuthMethod{},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         5 * time.Second,
		}

		var client *ssh.Client
		var err error
		if s.config.Proxy.Enabled {
			proxyDialer, dialErr := s.getProxyDialer()
			if dialErr != nil {
				s.addLog(conn, fmt.Sprintf("✗ 创建代理 Dialer 失败: %v", dialErr))
				continue
			}
			connProxy, dialErr := proxyDialer.Dial("tcp", addr)
			if dialErr != nil {
				s.addLog(conn, fmt.Sprintf("✗ 通过代理连接失败: %v", dialErr))
				continue
			}
			sshConn, chans, reqs, err := ssh.NewClientConn(connProxy, addr, config)
			if err == nil {
				client = ssh.NewClient(sshConn, chans, reqs)
			}
		} else {
			client, err = ssh.Dial("tcp", addr, config)
		}
		if err == nil {
			s.addLog(conn, fmt.Sprintf("✓ 用户 %s 密钥认证成功", user))
			s.addLog(conn, "执行命令: whoami, ip addr")
			result := s.executeSSHCommands(client)
			conn.Status = "success"
			conn.Message = fmt.Sprintf("连接成功（密钥认证或无密码，用户: %s）", user)
			conn.Result = result
			conn.ConnectedAt = time.Now()
			client.Close()
			return
		}
		s.addLog(conn, fmt.Sprintf("✗ 用户 %s 密钥认证失败: %v", user, err))
	}

	conn.Status = "failed"
	conn.Message = "连接失败: 所有尝试均失败"
	s.addLog(conn, "所有连接尝试均失败")
}

// executeSSHCommands 执行 SSH 命令
func (s *ConnectorService) executeSSHCommands(client *ssh.Client) string {
	var results []string

	commands := []string{"whoami", "ip addr"}
	for _, cmd := range commands {
		session, err := client.NewSession()
		if err != nil {
			results = append(results, fmt.Sprintf("命令 %s 执行失败: %v", cmd, err))
			continue
		}

		output, err := session.CombinedOutput(cmd)
		session.Close()

		if err != nil {
			results = append(results, fmt.Sprintf("命令 %s 执行失败: %v", cmd, err))
		} else {
			results = append(results, fmt.Sprintf("命令: %s\n%s", cmd, string(output)))
		}
	}

	return strings.Join(results, "")
}

// connectMongoDB 连接 MongoDB
func (s *ConnectorService) connectMongoDB(conn *models.Connection) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
		s.addLog(conn, "注意: MongoDB 连接暂不支持代理，将尝试直接连接")
	}

	var client *mongo.Client
	var err error
	var connected bool
	var username, password string

	// 如果用户提供了用户名和密码，直接使用，跳过未授权访问
	if conn.User != "" && conn.Pass != "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
		mongoURL := fmt.Sprintf("mongodb://%s:%s@%s:%s", conn.User, conn.Pass, conn.IP, conn.Port)
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
		if err == nil {
			err = client.Ping(ctx, nil)
			if err == nil {
				s.addLog(conn, "✓ 密码认证成功")
				username = conn.User
				password = conn.Pass
				connected = true
			} else {
				s.addLog(conn, fmt.Sprintf("✗ Ping 失败: %v", err))
				client.Disconnect(ctx)
			}
		} else {
			s.addLog(conn, fmt.Sprintf("✗ 连接失败: %v", err))
		}
		if !connected {
			conn.Status = "failed"
			conn.Message = fmt.Sprintf("连接失败: %v", err)
			s.addLog(conn, "密码认证失败")
			return
		}
	} else {
		// 尝试未授权访问
		s.addLog(conn, "尝试未授权访问（无认证）")
		mongoURL := fmt.Sprintf("mongodb://%s:%s", conn.IP, conn.Port)
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
		if err == nil {
			err = client.Ping(ctx, nil)
			if err == nil {
				s.addLog(conn, "✓ 未授权访问成功")
				connected = true
			} else {
				s.addLog(conn, fmt.Sprintf("✗ Ping 失败: %v", err))
				client.Disconnect(ctx)
			}
		} else {
			s.addLog(conn, fmt.Sprintf("✗ 连接失败: %v", err))
		}

		// 尝试使用用户名（无密码）
		if !connected && conn.User != "" {
			pass := conn.Pass
			if pass == "" {
				s.addLog(conn, fmt.Sprintf("尝试用户 %s 无密码连接", conn.User))
				mongoURL = fmt.Sprintf("mongodb://%s@%s:%s", conn.User, conn.IP, conn.Port)
			} else {
				s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
				mongoURL = fmt.Sprintf("mongodb://%s:%s@%s:%s", conn.User, conn.Pass, conn.IP, conn.Port)
			}
			client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
			if err == nil {
				err = client.Ping(ctx, nil)
				if err == nil {
					if pass == "" {
						s.addLog(conn, "✓ 无密码连接成功")
					} else {
						s.addLog(conn, "✓ 密码认证成功")
					}
					username = conn.User
					password = pass
					connected = true
				} else {
					s.addLog(conn, fmt.Sprintf("✗ Ping 失败: %v", err))
					client.Disconnect(ctx)
				}
			} else {
				s.addLog(conn, fmt.Sprintf("✗ 连接失败: %v", err))
			}
		}
	}

	if !connected {
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("连接失败: %v", err)
		s.addLog(conn, "所有连接尝试均失败")
		return
	}

	// 连接成功，执行 show dbs
	s.addLog(conn, "执行 show dbs")
	result := s.getMongoDBDatabases(client, ctx)
	conn.Status = "success"
	if username != "" && password != "" {
		conn.Message = "连接成功（使用用户名密码）"
	} else if username != "" {
		conn.Message = "连接成功（无密码）"
	} else {
		conn.Message = "连接成功（未授权访问）"
	}
	conn.Result = result
	conn.ConnectedAt = time.Now()
	client.Disconnect(ctx)
}

// getMongoDBDatabases 获取 MongoDB 数据库列表（相当于 show dbs）
func (s *ConnectorService) getMongoDBDatabases(client *mongo.Client, ctx context.Context) string {
	var results []string
	results = append(results, "数据库列表:")
	results = append(results, strings.Repeat("-", 50))

	// 使用 ListDatabaseNames 获取数据库列表
	databases, err := client.ListDatabaseNames(ctx, nil)
	if err != nil {
		results = append(results, fmt.Sprintf("获取数据库列表失败: %v", err))
		return strings.Join(results, "\n")
	}

	if len(databases) == 0 {
		results = append(results, "当前没有数据库")
	} else {
		results = append(results, fmt.Sprintf("共找到 %d 个数据库:", len(databases)))
		results = append(results, "")

		// 获取每个数据库的详细信息
		for _, dbName := range databases {
			// 跳过系统数据库（可选，根据需求决定是否显示）
			if dbName == "admin" || dbName == "local" || dbName == "config" {
				results = append(results, fmt.Sprintf("  %s (系统数据库)", dbName))
			} else {
				results = append(results, fmt.Sprintf("  %s", dbName))
			}

			// 尝试获取数据库统计信息
			db := client.Database(dbName)
			stats := db.RunCommand(ctx, map[string]interface{}{"dbStats": 1})
			if stats.Err() == nil {
				var statsResult map[string]interface{}
				if err := stats.Decode(&statsResult); err == nil {
					if size, ok := statsResult["dataSize"].(float64); ok {
						results = append(results, fmt.Sprintf("    数据大小: %.2f MB", size/1024/1024))
					}
					if collections, ok := statsResult["collections"].(float64); ok {
						results = append(results, fmt.Sprintf("    集合数: %.0f", collections))
					}
				}
			}
			results = append(results, "")
		}
	}

	return strings.Join(results, "\n")
}

// connectSMB 连接 SMB
func (s *ConnectorService) connectSMB(conn *models.Connection) {
	port := conn.Port
	if port == "" {
		port = "445" // SMB 默认端口
	}
	addr := net.JoinHostPort(conn.IP, port)
	s.addLog(conn, fmt.Sprintf("连接地址: %s", addr))
	
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
	}

	// 尝试连接
	connTCP, err := s.dialWithProxy("tcp", addr)
	if err != nil {
		s.addLog(conn, fmt.Sprintf("✗ TCP 连接失败: %v", err))
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("连接失败: %v", err)
		return
	}
	defer connTCP.Close()

	var session *smb2.Session
	var connected bool
	var loginType string

	// 如果用户提供了用户名和密码，直接使用
	if conn.User != "" && conn.Pass != "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
		d := &smb2.Dialer{
			Initiator: &smb2.NTLMInitiator{
				User:     conn.User,
				Password: conn.Pass,
			},
		}
		session, err = d.Dial(connTCP)
		if err == nil {
			s.addLog(conn, "✓ 密码认证成功")
			connected = true
			loginType = "使用用户名密码"
		} else {
			s.addLog(conn, fmt.Sprintf("✗ 密码认证失败: %v", err))
		}
		if !connected {
			conn.Status = "failed"
			conn.Message = fmt.Sprintf("连接失败: %v", err)
			s.addLog(conn, "密码认证失败")
			return
		}
	} else {
		// 尝试匿名访问（空用户名和密码）
		s.addLog(conn, "尝试匿名访问（空用户名和密码）")
		d := &smb2.Dialer{
			Initiator: &smb2.NTLMInitiator{
				User:     "",
				Password: "",
			},
		}
		session, err = d.Dial(connTCP)
		if err == nil {
			s.addLog(conn, "✓ 匿名访问成功")
			connected = true
			loginType = "匿名访问"
		} else {
			s.addLog(conn, fmt.Sprintf("✗ 匿名访问失败: %v", err))

			// 尝试使用提供的用户名（无密码）
			if conn.User != "" {
				s.addLog(conn, fmt.Sprintf("尝试用户 %s 无密码连接", conn.User))
				d := &smb2.Dialer{
					Initiator: &smb2.NTLMInitiator{
						User:     conn.User,
						Password: "",
					},
				}
				session, err = d.Dial(connTCP)
				if err == nil {
					s.addLog(conn, "✓ 无密码连接成功")
					connected = true
					loginType = "无密码"
				} else {
					s.addLog(conn, fmt.Sprintf("✗ 无密码连接失败: %v", err))
				}
			}
		}
	}

	if !connected {
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("连接失败: %v", err)
		s.addLog(conn, "所有连接尝试均失败")
		return
	}
	defer session.Logoff()

	// 连接成功，获取当前目录下的所有文件
	s.addLog(conn, "获取当前目录下的所有文件")
	result := s.getSMBFiles(session)
	conn.Status = "success"
	conn.Message = fmt.Sprintf("连接成功（%s）", loginType)
	conn.Result = result
	conn.ConnectedAt = time.Now()
}

// getSMBFiles 获取 SMB 共享中的文件列表
func (s *ConnectorService) getSMBFiles(session *smb2.Session) string {
	var results []string
	results = append(results, "文件列表:")
	results = append(results, strings.Repeat("-", 80))

	// 尝试常见的共享名称
	shares := []string{"C$", "IPC$", "ADMIN$", "Share", "Public", "共享"}
	if session != nil {
		// 首先尝试列出所有共享
		sharesList, err := session.ListSharenames()
		if err == nil && len(sharesList) > 0 {
			s.addLogForSMB(&results, fmt.Sprintf("发现 %d 个共享: %v", len(sharesList), sharesList))
			shares = sharesList
		}
	}

	// 尝试访问每个共享
	for _, shareName := range shares {
		fs, err := session.Mount(shareName)
		if err != nil {
			continue
		}

		results = append(results, "")
		results = append(results, fmt.Sprintf("共享: %s", shareName))
		results = append(results, strings.Repeat("-", 80))

		// 获取根目录的文件列表
		files, err := fs.ReadDir(".")
		if err != nil {
			results = append(results, fmt.Sprintf("读取目录失败: %v", err))
			fs.Umount()
			continue
		}

		if len(files) == 0 {
			results = append(results, "当前目录为空")
		} else {
			results = append(results, fmt.Sprintf("共找到 %d 个项目:", len(files)))
			results = append(results, "")
			results = append(results, fmt.Sprintf("%-10s %-15s %-20s %-30s", "类型", "大小", "修改时间", "名称"))
			results = append(results, strings.Repeat("-", 80))

			for _, file := range files {
				fileType := "文件"
				if file.IsDir() {
					fileType = "目录"
				}

				size := fmt.Sprintf("%d", file.Size())
				if file.IsDir() {
					size = "-"
				}

				timeStr := file.ModTime().Format("2006-01-02 15:04:05")
				if file.ModTime().IsZero() {
					timeStr = "-"
				}

				results = append(results, fmt.Sprintf("%-10s %-15s %-20s %-30s", fileType, size, timeStr, file.Name()))
			}
		}

		// 成功获取一个共享后卸载并返回
		fs.Umount()
		return strings.Join(results, "\n")
	}

	results = append(results, "无法访问任何共享")
	return strings.Join(results, "\n")
}

// addLogForSMB 为 SMB 结果添加日志（辅助函数）
func (s *ConnectorService) addLogForSMB(results *[]string, message string) {
	*results = append(*results, message)
}

// connectWMI 通过 wmic 获取网卡信息
func (s *ConnectorService) connectWMI(conn *models.Connection) {
	if runtime.GOOS != "windows" {
		msg := "当前系统不支持 WMI（仅支持 Windows 环境执行 wmic）"
		conn.Status = "failed"
		conn.Message = msg
		s.addLog(conn, msg)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := []string{}
	if conn.IP != "" {
		args = append(args, "/node:"+conn.IP)
	} else {
		s.addLog(conn, "未指定 IP，将默认本机")
	}

	if conn.User != "" {
		args = append(args, "/user:"+conn.User)
		if conn.Pass != "" {
			args = append(args, "/password:"+conn.Pass)
		} else {
			s.addLog(conn, "未提供密码，WMI 可能无法完成认证")
		}
	} else {
		s.addLog(conn, "未提供用户名，将使用当前系统上下文执行 wmic")
	}

	args = append(args, "nic", "get")
	s.addLog(conn, fmt.Sprintf("执行命令: wmic %s", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, "wmic", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := fmt.Sprintf("wmic 执行失败: %v", err)
		if stderr.Len() > 0 {
			errMsg = fmt.Sprintf("%s（%s）", errMsg, strings.TrimSpace(stderr.String()))
		}
		conn.Status = "failed"
		conn.Message = errMsg
		s.addLog(conn, errMsg)
		return
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = "命令执行成功，但未返回任何内容"
	}

	conn.Status = "success"
	conn.Message = "WMI 命令执行成功"
	conn.Result = output
	conn.ConnectedAt = time.Now()
	s.addLog(conn, "✓ WMI 命令执行成功")
}

// connectMQTT 连接 MQTT
func (s *ConnectorService) connectMQTT(conn *models.Connection) {
	port := conn.Port
	if port == "" {
		port = "1883" // MQTT 默认端口
	}
	addr := net.JoinHostPort(conn.IP, port)
	s.addLog(conn, fmt.Sprintf("连接地址: %s", addr))
	
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
		s.addLog(conn, "注意: MQTT 连接暂不支持代理，将尝试直接连接")
	}

	var client mqtt.Client
	var username, password string

	// 设置 MQTT 客户端选项
	opts := mqtt.NewClientOptions()
	opts.AddBroker(addr)
	opts.SetClientID(fmt.Sprintf("batch-connector-%d", time.Now().UnixNano()))
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetAutoReconnect(false)
	opts.SetCleanSession(true)

	// 如果用户提供了用户名和密码，直接使用
	if conn.User != "" && conn.Pass != "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", conn.User))
		opts.SetUsername(conn.User)
		opts.SetPassword(conn.Pass)
		username = conn.User
		password = conn.Pass
	} else if conn.User != "" {
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 无密码连接", conn.User))
		opts.SetUsername(conn.User)
		username = conn.User
	} else {
		s.addLog(conn, "尝试未授权访问（无用户名密码）")
	}

	// 创建客户端
	client = mqtt.NewClient(opts)

	// 尝试连接
	s.addLog(conn, "正在连接 MQTT Broker...")
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		s.addLog(conn, fmt.Sprintf("✗ MQTT 连接失败: %v", token.Error()))
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("连接失败: %v", token.Error())
		return
	}

	// 检查连接状态
	if !client.IsConnected() {
		s.addLog(conn, "✗ MQTT 连接失败: 连接超时")
		conn.Status = "failed"
		conn.Message = "连接失败: 连接超时"
		client.Disconnect(250)
		return
	}

	s.addLog(conn, "✓ MQTT 连接成功")
	s.addLog(conn, "获取 MQTT Broker 基础信息")

	// 获取 MQTT 基础信息
	result := s.getMQTTInfo(client, addr, username)
	conn.Status = "success"
	if username != "" {
		if password != "" {
			conn.Message = fmt.Sprintf("连接成功（用户: %s）", username)
		} else {
			conn.Message = fmt.Sprintf("连接成功（用户: %s，无密码）", username)
		}
	} else {
		conn.Message = "连接成功（未授权访问）"
	}
	conn.Result = result
	conn.ConnectedAt = time.Now()

	// 断开连接
	client.Disconnect(250)
}

// getMQTTInfo 获取 MQTT Broker 基础信息
func (s *ConnectorService) getMQTTInfo(client mqtt.Client, addr, username string) string {
	var results []string
	results = append(results, "MQTT Broker 基础信息:")
	results = append(results, strings.Repeat("-", 50))

	// Broker 地址
	results = append(results, fmt.Sprintf("Broker 地址: %s", addr))

	// 客户端 ID
	if client != nil && client.IsConnected() {
		results = append(results, "连接状态: 已连接")
	} else {
		results = append(results, "连接状态: 未连接")
	}

	// 用户名
	if username != "" {
		results = append(results, fmt.Sprintf("认证用户: %s", username))
	} else {
		results = append(results, "认证用户: 无（未授权访问）")
	}

	// 尝试订阅系统主题获取版本信息（如果支持）
	results = append(results, "")
	results = append(results, "尝试获取 Broker 信息...")

	// 尝试订阅 $SYS 主题（很多 MQTT Broker 支持）
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 尝试获取一些系统主题信息
	sysTopics := []string{
		"$SYS/broker/version",
		"$SYS/broker/uptime",
		"$SYS/broker/clients/connected",
	}

	var receivedInfo []string
	messageReceived := make(chan bool, 1)

	for _, topic := range sysTopics {
		token := client.Subscribe(topic, 0, func(c mqtt.Client, msg mqtt.Message) {
			receivedInfo = append(receivedInfo, fmt.Sprintf("  %s: %s", msg.Topic(), string(msg.Payload())))
			messageReceived <- true
		})

		if token.WaitTimeout(1*time.Second) && token.Error() == nil {
			// 等待消息或超时
			select {
			case <-messageReceived:
				// 收到消息，继续
			case <-ctx.Done():
				// 超时，继续下一个
			}
			client.Unsubscribe(topic)
		}
	}

	if len(receivedInfo) > 0 {
		results = append(results, "系统主题信息:")
		results = append(results, receivedInfo...)
	} else {
		results = append(results, "未获取到系统主题信息（Broker 可能不支持 $SYS 主题）")
	}

	// 测试发布和订阅功能
	results = append(results, "")
	results = append(results, "功能测试:")
	testTopic := fmt.Sprintf("test/batch-connector/%d", time.Now().UnixNano())
	testMessage := "test message from batch-connector"

	// 订阅测试主题
	var testReceived bool
	subToken := client.Subscribe(testTopic, 0, func(c mqtt.Client, msg mqtt.Message) {
		testReceived = true
	})

	if subToken.WaitTimeout(1*time.Second) && subToken.Error() == nil {
		results = append(results, fmt.Sprintf("  订阅功能: 正常（主题: %s）", testTopic))

		// 发布测试消息
		pubToken := client.Publish(testTopic, 0, false, testMessage)
		if pubToken.WaitTimeout(1*time.Second) && pubToken.Error() == nil {
			// 等待消息接收
			time.Sleep(500 * time.Millisecond)
			if testReceived {
				results = append(results, "  发布/订阅功能: 正常")
			} else {
				results = append(results, "  发布功能: 正常（但未收到订阅消息）")
			}
		} else {
			results = append(results, fmt.Sprintf("  发布功能: 失败 (%v)", pubToken.Error()))
		}

		// 取消订阅
		client.Unsubscribe(testTopic)
	} else {
		results = append(results, fmt.Sprintf("  订阅功能: 失败 (%v)", subToken.Error()))
	}

	return strings.Join(results, "\n")
}

// connectOracle 连接 Oracle 数据库（使用纯 Go 实现的 go-ora 驱动，无需 Oracle Instant Client）
func (s *ConnectorService) connectOracle(conn *models.Connection) {
	port := conn.Port
	if port == "" {
		port = "1521" // Oracle 默认端口
	}
	addr := net.JoinHostPort(conn.IP, port)
	s.addLog(conn, fmt.Sprintf("目标 Oracle 数据库地址: %s", addr))
	
	// 检查是否使用代理
	if s.config.Proxy.Enabled {
		s.addLog(conn, fmt.Sprintf("使用 SOCKS5 代理: %s:%s", s.config.Proxy.Host, s.config.Proxy.Port))
		s.addLog(conn, "注意: Oracle 连接暂不支持代理，将尝试直接连接")
	}

	portInt := 1521
	if port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			portInt = p
		}
	}

	// 常见的 Oracle 服务名列表
	serviceNames := []string{"XE", "ORCL", "XEPDB1", "ORCLPDB", "ORCLCDB", "PDBORCL"}

	var username, password string
	// 确定要尝试的用户名和密码
	if conn.User != "" && conn.Pass != "" {
		username = conn.User
		password = conn.Pass
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 密码认证", username))
	} else if conn.User != "" {
		username = conn.User
		password = ""
		s.addLog(conn, fmt.Sprintf("尝试用户 %s 无密码连接", username))
	} else {
		// 尝试常见的默认用户组合
		username = "sys"
		password = "system"
		s.addLog(conn, "尝试默认用户 sys/system 连接")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var db *sql.DB
	var err error
	var successServiceName string
	var successUsername string
	var successPassword string

	// 尝试不同的服务名
	for _, serviceName := range serviceNames {
		s.addLog(conn, fmt.Sprintf("尝试服务名: %s", serviceName))
		
		var dsn string
		if password != "" {
			dsn = go_ora.BuildUrl(conn.IP, portInt, serviceName, username, password, nil)
		} else {
			dsn = go_ora.BuildUrl(conn.IP, portInt, serviceName, username, "", nil)
		}

		db, err = sql.Open("oracle", dsn)
		if err != nil {
			s.addLog(conn, fmt.Sprintf("  创建连接失败: %v", err))
			continue
		}

		// 测试连接
		err = db.PingContext(ctx)
		if err != nil {
			errMsg := err.Error()
			// 如果是服务名错误，尝试下一个服务名
			if strings.Contains(errMsg, "ORA-12514") || strings.Contains(errMsg, "TNS:listener does not currently know of service") {
				s.addLog(conn, fmt.Sprintf("  服务名 %s 不存在，尝试下一个", serviceName))
				db.Close()
				continue
			}
			// 如果是认证失败，尝试下一个服务名（可能是服务名不对）
			if strings.Contains(errMsg, "ORA-01017") || strings.Contains(errMsg, "invalid username/password") {
				s.addLog(conn, fmt.Sprintf("  认证失败，服务名 %s 可能不正确，尝试下一个", serviceName))
				db.Close()
				continue
			}
			// 其他错误，也尝试下一个服务名
			s.addLog(conn, fmt.Sprintf("  连接失败: %v，尝试下一个服务名", err))
			db.Close()
			continue
		}

		// 连接成功
		successServiceName = serviceName
		successUsername = username
		successPassword = password
		s.addLog(conn, fmt.Sprintf("✓ 使用服务名 %s 连接成功", serviceName))
		err = nil // 标记连接成功
		break
	}

	// 如果所有服务名都失败，且使用的是默认用户，尝试其他用户组合
	if err != nil && successServiceName == "" && username == "sys" && password == "system" {
		s.addLog(conn, "尝试默认用户 scott/tiger 连接")
		username = "scott"
		password = "tiger"
		
		for _, serviceName := range serviceNames {
			s.addLog(conn, fmt.Sprintf("尝试服务名: %s (用户: scott/tiger)", serviceName))
			dsn := go_ora.BuildUrl(conn.IP, portInt, serviceName, username, password, nil)
			
			db, err = sql.Open("oracle", dsn)
			if err != nil {
				continue
			}

			err = db.PingContext(ctx)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "ORA-12514") || strings.Contains(errMsg, "TNS:listener does not currently know of service") {
					db.Close()
					continue
				}
				if strings.Contains(errMsg, "ORA-01017") || strings.Contains(errMsg, "invalid username/password") {
					db.Close()
					continue
				}
				db.Close()
				continue
			}

			// 连接成功
			successServiceName = serviceName
			successUsername = username
			successPassword = password
			s.addLog(conn, fmt.Sprintf("✓ 使用服务名 %s 和用户 scott/tiger 连接成功", serviceName))
			err = nil // 标记连接成功
			break
		}
	}

	// 如果所有尝试都失败
	if err != nil {
		s.addLog(conn, fmt.Sprintf("✗ Oracle 连接失败: %v", err))
		s.addLog(conn, "提示: 已尝试常见服务名 (XE, ORCL, XEPDB1, ORCLPDB, ORCLCDB, PDBORCL)")
		s.addLog(conn, "如果您的数据库使用其他服务名，请检查 Oracle 监听器配置")
		if db != nil {
			db.Close()
		}
		conn.Status = "failed"
		conn.Message = fmt.Sprintf("连接失败: %v", err)
		return
	}

	// 更新用户名和密码变量
	username = successUsername
	password = successPassword

	s.addLog(conn, fmt.Sprintf("✓ Oracle 连接成功 (服务名: %s, 用户: %s)", successServiceName, username))
	s.addLog(conn, "执行查询: SELECT name FROM v$database")

	// 获取数据库信息
	result := s.getOracleDatabases(db, ctx)
	conn.Status = "success"
	if username != "" {
		if password != "" {
			conn.Message = fmt.Sprintf("连接成功（用户: %s）", username)
		} else {
			conn.Message = fmt.Sprintf("连接成功（用户: %s，无密码）", username)
		}
	} else {
		conn.Message = "连接成功"
	}
	conn.Result = result
	conn.ConnectedAt = time.Now()
	db.Close()
}

// getOracleDatabases 获取 Oracle 数据库信息
func (s *ConnectorService) getOracleDatabases(db *sql.DB, ctx context.Context) string {
	var results []string
	results = append(results, "数据库信息:")
	results = append(results, strings.Repeat("-", 50))

	// 执行查询: SELECT name FROM v$database
	query := "SELECT name FROM v$database"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		results = append(results, fmt.Sprintf("查询失败: %v", err))
		// 尝试其他查询获取数据库信息
		results = append(results, "")
		results = append(results, "尝试获取其他数据库信息...")

		// 尝试查询实例信息
		altQuery := "SELECT instance_name, host_name, version FROM v$instance"
		altRows, altErr := db.QueryContext(ctx, altQuery)
		if altErr == nil {
			defer altRows.Close()
			results = append(results, "实例信息:")
			for altRows.Next() {
				var instanceName, hostName, version string
				if err := altRows.Scan(&instanceName, &hostName, &version); err == nil {
					results = append(results, fmt.Sprintf("  实例名: %s", instanceName))
					results = append(results, fmt.Sprintf("  主机名: %s", hostName))
					results = append(results, fmt.Sprintf("  版本: %s", version))
				}
			}
		}
		return strings.Join(results, "\n")
	}
	defer rows.Close()

	results = append(results, "数据库名称:")
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			results = append(results, fmt.Sprintf("读取行失败: %v", err))
			continue
		}
		results = append(results, fmt.Sprintf("  - %s", name))
	}

	if err := rows.Err(); err != nil {
		results = append(results, fmt.Sprintf("遍历行时出错: %v", err))
	}

	// 尝试获取更多信息
	results = append(results, "")
	results = append(results, "实例信息:")
	instanceQuery := "SELECT instance_name, host_name, version FROM v$instance"
	instanceRows, err := db.QueryContext(ctx, instanceQuery)
	if err == nil {
		defer instanceRows.Close()
		for instanceRows.Next() {
			var instanceName, hostName, version string
			if err := instanceRows.Scan(&instanceName, &hostName, &version); err == nil {
				results = append(results, fmt.Sprintf("  实例名: %s", instanceName))
				results = append(results, fmt.Sprintf("  主机名: %s", hostName))
				results = append(results, fmt.Sprintf("  版本: %s", version))
			}
		}
	}

	return strings.Join(results, "\n")
}
