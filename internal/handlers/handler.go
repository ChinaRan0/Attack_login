package handlers

import (
	"batch-connector/internal/config"
	"batch-connector/internal/models"
	"batch-connector/internal/services"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *services.ConnectorService
	config  *config.Config
}

func NewHandler(service *services.ConnectorService) *Handler {
	cfg, _ := config.LoadConfig()
	return &Handler{
		service: service,
		config:  cfg,
	}
}

// LoginPage 登录页面
func (h *Handler) LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "登录 - Attack_login",
	})
}

// Login 登录验证
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if req.Password != h.config.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	// 设置 session cookie
	c.SetCookie("authenticated", "true", 3600*24*7, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "登录成功"})
}

// Logout 登出
func (h *Handler) Logout(c *gin.Context) {
	c.SetCookie("authenticated", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

// Index 首页
func (h *Handler) Index(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Attack_login",
	})
}

// ImportCSV 导入 CSV
func (h *Handler) ImportCSV(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败: " + err.Error()})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件打开失败: " + err.Error()})
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV 解析失败: " + err.Error()})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV 文件至少需要包含表头和数据行"})
		return
	}

	// 解析表头
	header := records[0]
	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	// 检查必需的列
	requiredFields := []string{"type", "ip", "port"}
	for _, field := range requiredFields {
		if _, exists := headerMap[field]; !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "CSV 文件缺少必需的列: " + field})
			return
		}
	}

	var connections []*models.Connection
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) <= headerMap["type"] || len(record) <= headerMap["ip"] || len(record) <= headerMap["port"] {
			continue
		}

		connType := strings.TrimSpace(record[headerMap["type"]])
		ip := strings.TrimSpace(record[headerMap["ip"]])
		port := strings.TrimSpace(record[headerMap["port"]])

		if connType == "" || ip == "" || port == "" {
			continue
		}

		user := ""
		pass := ""
		if idx, exists := headerMap["user"]; exists && idx < len(record) {
			user = strings.TrimSpace(record[idx])
		}
		if idx, exists := headerMap["pass"]; exists && idx < len(record) {
			pass = strings.TrimSpace(record[idx])
		}

		conn := h.service.CreateConnectionFromCSV(connType, ip, port, user, pass)
		if err := h.service.AddConnection(conn); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存连接失败: " + err.Error()})
			return
		}
		connections = append(connections, conn)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "导入成功",
		"count":       len(connections),
		"connections": connections,
	})
}

// Connect 单个连接
func (h *Handler) Connect(c *gin.Context) {
	// 读取原始请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败: " + err.Error()})
		return
	}

	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体为空"})
		return
	}

	// 先解析为 map 来判断请求类型
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON 解析失败: " + err.Error()})
		return
	}

	// 如果包含 id 字段且只有 id 字段，则是重新连接
	if idVal, hasID := data["id"]; hasID {
		id, ok := idVal.(string)
		if ok && id != "" {
			// 通过 ID 重新连接
			conn, exists := h.service.GetConnection(id)
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "连接不存在"})
				return
			}
			// 异步执行连接
			go h.service.Connect(conn)
			c.JSON(http.StatusOK, gin.H{
				"message":    "连接任务已启动",
				"connection": conn,
			})
			return
		}
	}

	// 否则解析为新建连接请求
	var req models.ConnectionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证必需字段
	if req.Type == "" || req.IP == "" || req.Port == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必需字段: type, ip, port"})
		return
	}

	conn := h.service.CreateConnectionFromCSV(req.Type, req.IP, req.Port, req.User, req.Pass)
	if err := h.service.AddConnection(conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存连接失败: " + err.Error()})
		return
	}

	// 异步执行连接
	go h.service.Connect(conn)

	c.JSON(http.StatusOK, gin.H{
		"message":    "连接任务已启动",
		"connection": conn,
	})
}

// ConnectBatch 批量连接
func (h *Handler) ConnectBatch(c *gin.Context) {
	var req models.BatchConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var connections []*models.Connection
	for _, id := range req.IDs {
		conn, exists := h.service.GetConnection(id)
		if !exists {
			continue
		}
		connections = append(connections, conn)
		// 异步执行连接
		go h.service.Connect(conn)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "批量连接任务已启动",
		"count":       len(connections),
		"connections": connections,
	})
}

// GetConnections 获取所有连接
func (h *Handler) GetConnections(c *gin.Context) {
	connType := c.Query("type")
	port := strings.TrimSpace(c.Query("port"))
	user := strings.TrimSpace(c.Query("user"))
	status := strings.TrimSpace(c.Query("status"))
	message := strings.TrimSpace(c.Query("message"))

	var connections []*models.Connection

	if connType != "" {
		connections = h.service.GetConnectionsByType(connType)
	} else {
		connections = h.service.GetAllConnections()
	}

	// 按照筛选条件过滤
	filtered := make([]*models.Connection, 0, len(connections))
	for _, conn := range connections {
		if port != "" && conn.Port != port {
			continue
		}
		if user != "" {
			if conn.User == "" || !strings.Contains(strings.ToLower(conn.User), strings.ToLower(user)) {
				continue
			}
		}
		if status != "" && !strings.EqualFold(conn.Status, status) {
			continue
		}
		if message != "" {
			msg := strings.ToLower(conn.Message)
			if msg == "" || !strings.Contains(msg, strings.ToLower(message)) {
				continue
			}
		}
		filtered = append(filtered, conn)
	}

	c.JSON(http.StatusOK, gin.H{
		"connections": filtered,
		"count":       len(filtered),
	})
}

// DeleteConnection 删除连接
func (h *Handler) DeleteConnection(c *gin.Context) {
	id := c.Param("id")
	if h.service.DeleteConnection(id) {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "连接不存在"})
	}
}

// DeleteBatchConnections 批量删除连接
func (h *Handler) DeleteBatchConnections(c *gin.Context) {
	var req models.BatchConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要删除的连接"})
		return
	}

	count, err := h.service.DeleteBatchConnections(req.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("成功删除 %d 条连接记录", count),
		"count":   count,
	})
}

// UpdateConnection 更新连接信息
func (h *Handler) UpdateConnection(c *gin.Context) {
	id := c.Param("id")

	var req models.ConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证必需字段
	if req.Type == "" || req.IP == "" || req.Port == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必需字段: type, ip, port"})
		return
	}

	// 检查连接是否存在
	existingConn, exists := h.service.GetConnection(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "连接不存在"})
		return
	}

	// 如果密码为空，保留原密码
	password := req.Pass
	if password == "" {
		password = existingConn.Pass
	}

	// 更新连接信息
	if err := h.service.UpdateConnectionInfo(id, req.Type, req.IP, req.Port, req.User, password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新连接失败: " + err.Error()})
		return
	}

	// 获取更新后的连接
	conn, _ := h.service.GetConnection(id)

	c.JSON(http.StatusOK, gin.H{
		"message":    "连接更新成功",
		"connection": conn,
	})
}

// GetProxySettings 获取代理配置
func (h *Handler) GetProxySettings(c *gin.Context) {
	cfg := config.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载配置"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"proxy": cfg.Proxy,
	})
}

// UpdateProxySettings 更新代理配置
func (h *Handler) UpdateProxySettings(c *gin.Context) {
	var req config.ProxyConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Type == "" {
		req.Type = "socks5"
	}

	if req.Enabled {
		if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Port) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "启用代理时必须填写主机和端口"})
			return
		}
	}

	current := config.GetConfig()
	if current == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载配置"})
		return
	}

	updated := *current
	updated.Proxy = req

	if err := config.SaveConfig(&updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	h.config = &updated
	h.service.UpdateConfig(&updated)

	c.JSON(http.StatusOK, gin.H{
		"message": "代理配置已更新",
		"proxy":   updated.Proxy,
	})
}
