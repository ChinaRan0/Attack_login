# Attack_login - 批量连接测试工具


一个基于 Golang 开发的 Web 批量连接测试工具，专为红队攻防演练和安全测试设计，支持批量测试多种服务的连接状态和未授权访问检测。

下载：

https://pan.quark.cn/s/e375e4fc07f5



**公众号：知攻善防实验室**  
**开发者：ChinaRan404**


---

## 📋 目录

- [功能特性](#功能特性)
- [快速开始](#快速开始)
- [详细使用说明](#详细使用说明)
- [技术架构原理](#技术架构原理)
- [支持的协议和服务](#支持的协议和服务)
- [配置说明](#配置说明)
- [项目结构](#项目结构)
- [注意事项](#注意事项)

---

## ✨ 功能特性

- ✅ **多协议支持**：支持 14 种常见服务类型的连接测试（新增 Zookeeper + Elasticsearch）
- ✅ **批量操作**：支持 CSV 批量导入和批量连接测试
- ✅ **未授权检测**：自动检测常见服务的未授权访问漏洞
- ✅ **实时状态**：实时显示连接状态和详细日志
- ✅ **分类管理**：按服务类型分类显示和管理
- ✅ **Web 界面**：友好的中文 Web 界面，无需命令行操作
- ✅ **安全认证**：支持密码保护，防止未授权访问
- ✅ **代理穿透**：内置 SOCKS5 全局代理设置，可在前端直接配置
- ✅ **SSH 命令执行**：SSH 连接成功后自动执行系统命令
- ✅ **跨平台支持**：支持 Windows、Linux、macOS 多平台
- ✅ **纯 Go 依赖**：SQLite 与 Oracle 均使用纯 Go 驱动，无需额外客户端

---

## 🚀 快速开始

### 前置要求

- Go 1.21 或更高版本（仅开发环境需要）
- 或直接使用编译好的二进制文件

### 方式一：使用编译好的二进制文件

1. 从 [Releases](https://github.com/ChinaRan0/Attack_login/releases) 下载对应平台的压缩包
2. 解压后运行可执行文件
3. 访问 `http://localhost:18921`

### 方式二：从源码编译

```bash
# 克隆项目
git clone https://github.com/ChinaRan0/Attack_login.git
cd attack_login

# 安装依赖
go mod download

# 运行
go run main.go
```

服务器将在 `http://localhost:18921` 启动。

### 首次登录

1. 访问 `http://localhost:18921`
2. 系统会自动跳转到登录页面
3. 默认密码在 `config.json` 文件中配置（默认：`admin123`）
4. 首次登录后会显示使用须知

---

## 📖 详细使用说明

### 1. 登录系统

访问系统后，输入配置文件中设置的密码即可登录。登录状态会保存在 Cookie 中，7 天内有效。

### 2. CSV 批量导入

#### CSV 文件格式

CSV 文件必须包含以下列（表头必须存在）：

| 列名 | 说明 | 必填 | 示例 |
|------|------|------|------|
| Type | 服务类型 | ✅ | Redis, MySQL, SSH |
| IP | IP 地址 | ✅ | 192.168.1.100 |
| Port | 端口号 | ✅ | 3306 |
| User | 用户名 | ❌ | root |
| Pass | 密码 | ❌ | password123 |

#### 示例 CSV 文件

```csv
Type,IP,Port,User,Pass
Redis,192.168.1.100,6379,,
MySQL,192.168.1.101,3306,root,password123
SSH,192.168.1.102,22,admin,admin123
PostgreSQL,192.168.1.103,5432,postgres,
MongoDB,192.168.1.104,27017,,
SQLServer,192.168.1.107,1433,sa,Password123!
RabbitMQ,192.168.1.105,5672,guest,guest
SMB,192.168.1.106,445,administrator,P@ssw0rd!
WMI,192.168.1.120,,administrator,P@ssw0rd!
MQTT,192.168.1.108,1883,admin,admin123
Oracle,192.168.1.109,1521,scott,tiger
Elasticsearch,192.168.1.150,9200,,
Zookeeper,192.168.1.151,2181,admin,secret
```

#### 导入步骤

1. 点击顶部工具栏的 **"导入 CSV"** 按钮
2. 选择准备好的 CSV 文件
3. 点击上传，系统会自动解析并导入所有连接记录
4. 导入成功后，连接记录会显示在连接列表中

### 3. 手动添加连接

1. 点击顶部工具栏的 **"添加连接"** 按钮
2. 选择服务类型（选择后会自动显示默认端口和默认账户提示；Elasticsearch 可直接填写 `https://host:9200/_cat/indices?pretty`；Zookeeper 可填写多个节点 `10.0.0.1,10.0.0.2` 并自动补全端口）
3. 填写 IP 地址和端口
4. 可选填写用户名和密码（留空会尝试未授权访问）
5. 点击 **"添加并连接"** 按钮
6. 系统会立即尝试连接并显示结果

### 4. 连接测试

#### 单个连接测试

- 在连接列表中点击 **"重连"** 按钮
- 系统会重新尝试连接并更新状态

#### 批量连接测试

1. 在连接列表中勾选需要测试的连接（可使用表头的全选复选框）
2. 点击 **"批量连接选中"** 按钮
3. 系统会异步执行所有连接测试
4. 页面会自动刷新显示最新状态

### 5. 查看连接详情

1. 在连接列表中点击 **"详情"** 按钮
2. 展开后可以查看：
   - 详细的连接日志
   - 命令执行结果（SSH 等）
   - 数据库信息（数据库类型服务）

### 6. 筛选和搜索

- **按类型筛选**：点击左侧边栏的服务类型，只显示该类型的连接
- **高级筛选**：使用顶部的筛选栏，可按端口、用户名、状态、消息内容筛选
- **重置筛选**：点击 **"重置"** 按钮清除所有筛选条件

### 7. 编辑和删除

- **编辑连接**：点击 **"编辑"** 按钮，修改连接信息后保存
- **删除连接**：点击 **"删除"** 按钮，确认后删除
- **批量删除**：勾选多个连接后，点击 **"批量删除选中"** 按钮

---

## 🏗️ 技术架构原理

### 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                      Web 前端层                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │ HTML模板 │  │  CSS样式 │  │ JavaScript│             │
│  └──────────┘  └──────────┘  └──────────┘             │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTP/HTTPS
                       │ RESTful API
┌──────────────────────▼──────────────────────────────────┐
│                    HTTP 服务层                           │
│  ┌──────────────────────────────────────────┐         │
│  │         Gin Web Framework                 │         │
│  │  - 路由管理                               │         │
│  │  - 中间件（认证、CORS等）                 │         │
│  │  - 请求处理                               │         │
│  └──────────────────────────────────────────┘         │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                   业务逻辑层                             │
│  ┌──────────────────────────────────────────┐         │
│  │      Handlers (HTTP 处理器)               │         │
│  │  - 请求解析和验证                         │         │
│  │  - 调用 Service 层                        │         │
│  │  - 响应格式化                             │         │
│  └──────────────────────────────────────────┘         │
│  ┌──────────────────────────────────────────┐         │
│  │      Services (业务服务)                  │         │
│  │  - ConnectorService: 连接管理             │         │
│  │  - 连接测试逻辑                           │         │
│  │  - 异步任务调度                           │         │
│  └──────────────────────────────────────────┘         │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                   数据持久层                             │
│  ┌──────────────────────────────────────────┐         │
│  │      SQLite 数据库                        │         │
│  │  - 连接信息存储                           │         │
│  │  - 状态和日志记录                         │         │
│  │  - WAL 模式（写前日志）                   │         │
│  └──────────────────────────────────────────┘         │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                   协议连接层                             │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐        │
│  │Redis │ │MySQL │ │PostgreSQL│SSH │ │MongoDB│        │
│  └──────┘ └──────┘ └──────┘ └──────┘ └──────┘        │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐        │
│  │ FTP  │ │SMB   │ │WMI   │ │MQTT  │ │Oracle│        │
│  └──────┘ └──────┘ └──────┘ └──────┘ └──────┘        │
└─────────────────────────────────────────────────────────┘
```

### 核心组件说明

#### 1. Web 前端（Frontend）

- **技术栈**：原生 HTML + CSS + JavaScript（无框架依赖）
- **特点**：
  - 轻量级，无需构建工具
  - 响应式设计，适配不同屏幕
  - 实时数据刷新（3秒轮询）
  - 本地存储（localStorage）保存用户偏好

#### 2. HTTP 服务层（Gin Framework）

- **路由管理**：
  ```go
  // 公开路由
  r.GET("/login", handler.LoginPage)
  r.POST("/api/login", handler.Login)
  
  // 需要认证的路由组
  authorized := r.Group("/")
  authorized.Use(authMiddleware())
  ```

- **认证机制**：
  - Cookie 基础的会话管理
  - 中间件拦截未授权请求
  - 密码从配置文件读取（`config.json`）

#### 3. 业务逻辑层（Services）

**ConnectorService 核心功能**：

```go
type ConnectorService struct {
    db *sql.DB  // SQLite 数据库连接
}
```

- **连接管理**：
  - `AddConnection()`: 添加连接记录
  - `GetConnection()`: 获取单个连接
  - `GetAllConnections()`: 获取所有连接
  - `UpdateConnection()`: 更新连接状态

- **连接测试**：
  - `Connect()`: 执行连接测试（异步）
  - 根据服务类型调用对应的连接函数
  - 自动尝试未授权访问

#### 4. 数据持久层（SQLite）

- **数据库设计**：
  ```sql
  CREATE TABLE connections (
      id TEXT PRIMARY KEY,           -- UUID
      type TEXT NOT NULL,            -- 服务类型
      ip TEXT NOT NULL,              -- IP 地址
      port TEXT NOT NULL,            -- 端口
      user TEXT,                     -- 用户名
      pass TEXT,                     -- 密码
      status TEXT NOT NULL,          -- 状态：pending/success/failed
      message TEXT,                  -- 连接结果消息
      result TEXT,                   -- 详细信息（SSH命令结果等）
      logs TEXT,                     -- JSON 格式的日志数组
      created_at TEXT NOT NULL,      -- 创建时间
      connected_at TEXT              -- 连接成功时间
  );
  ```

- **特性**：
  - 纯 Go 驱动（modernc.org/sqlite），避免 CGO 依赖
  - WAL 模式（Write-Ahead Logging）提高并发性能
  - 索引优化查询性能
  - 自动创建数据库文件

#### 5. 协议连接层（Protocol Connectors）

每种服务类型都有独立的连接函数：

- **Redis**: 使用 `github.com/go-redis/redis/v8`
- **MySQL**: 使用 `github.com/go-sql-driver/mysql`
- **PostgreSQL**: 使用 `github.com/lib/pq`
- **SSH**: 使用 `golang.org/x/crypto/ssh`
- **MongoDB**: 使用 `go.mongodb.org/mongo-driver`
- **Oracle**: 使用 `github.com/sijms/go-ora/v2`（纯 Go 实现，无需 Instant Client）
- **Zookeeper**: 使用 `github.com/samuel/go-zookeeper/zk`（支持多节点、digest 认证、SOCKS5 代理，默认执行 `ls /`）
- **其他协议**: 使用对应的 Go 客户端库

### 异步连接机制

```go
// 连接测试采用异步执行
go h.service.Connect(conn)
```

- **优势**：
  - 不阻塞 HTTP 请求
  - 支持批量并发连接
  - 实时更新状态到数据库

- **状态流转**：
  ```
  pending → success/failed
  ```

### 未授权访问检测逻辑

每种服务都有特定的未授权检测策略：

1. **Redis**: 尝试无密码连接
2. **FTP**: 尝试匿名登录（anonymous/anonymous）
3. **MySQL**: 尝试 root 用户无密码
4. **PostgreSQL**: 尝试 postgres 用户无密码
5. **MongoDB**: 尝试无认证连接
6. **Oracle**: 尝试 sys/system 或 scott/tiger
7. **Zookeeper**: 尝试无 ACL 访问（无 digest 认证）

### 配置管理

- **配置文件**：`config.json`
  ```json
  {
    "password": "admin123",
    "port": "18921",
    "proxy": {
      "enabled": false,
      "type": "socks5",
      "host": "127.0.0.1",
      "port": "1080",
      "user": "",
      "pass": ""
    }
  }
  ```

- **配置加载**：使用单例模式，首次加载后缓存
- **前端管理**：登录后点击“代理设置”即可实时修改 SOCKS5 配置（无需重启）

---

## 🔌 支持的协议和服务

### 数据库服务

| 服务 | 默认端口 | 默认账户 | 未授权检测 |
|------|---------|---------|-----------|
| MySQL | 3306 | root | root 无密码 |
| PostgreSQL | 5432 | postgres | postgres 无密码 |
| SQL Server | 1433 | sa | sa 账户 |
| Oracle | 1521 | sys/system, scott/tiger | 自动尝试多个服务名 |
| MongoDB | 27017 | - | 无认证连接 |
| Redis | 6379 | - | 无密码连接 |

### 文件传输服务

| 服务 | 默认端口 | 默认账户 | 未授权检测 |
|------|---------|---------|-----------|
| FTP | 21 | anonymous | anonymous/anonymous |
| SMB | 445 | administrator | administrator 账户 |
| SSH | 22 | root, admin | 密钥认证或无密码 |

### 消息队列服务

| 服务 | 默认端口 | 默认账户 | 未授权检测 |
|------|---------|---------|-----------|
| RabbitMQ | 5672 | guest | guest/guest |
| MQTT | 1883 | admin | admin/admin 或无认证 |

### 搜索/监控服务

| 服务 | 默认端口 | 默认路径 | 说明 |
|------|---------|---------|------|
| Elasticsearch | 9200 | `/_cat/indices?pretty` | 可自定义路径 & Basic Auth，HTTP(S)+SOCKS5 |

### 注册 / 协调服务

| 服务 | 默认端口 | 默认命令 | 说明 |
|------|---------|---------|------|
| Zookeeper | 2181 | `ls /` | 支持多节点输入、digest 认证、SOCKS5 代理 |

#### Zookeeper 详细说明

**多节点支持**：
- 在 IP 地址字段可以填写多个节点，使用逗号、分号或空格分隔
- 示例：`10.0.0.1,10.0.0.2,10.0.0.3` 或 `10.0.0.1:2181;10.0.0.2:2181`
- 如果只提供 IP 地址（不含端口），会自动使用默认端口 2181
- 如果节点已包含端口号，则使用指定的端口

**认证方式**：
- 支持 digest 认证（用户名:密码格式）
- 如果提供了用户名但未提供密码，会尝试无密码 digest 认证
- 留空用户名和密码表示尝试未授权访问（无 ACL）

**连接特性**：
- 自动建立 ZooKeeper 会话（会话超时 10 秒）
- 连接成功后自动执行 `ls /` 命令，列出根节点下的所有子节点
- 显示节点统计信息（czxid、mzxid、version、创建时间、修改时间）
- 支持 SOCKS5 代理连接
- 连接超时时间：15 秒

**使用示例**：
- 单节点：`192.168.1.100` 或 `192.168.1.100:2181`
- 多节点：`192.168.1.100,192.168.1.101,192.168.1.102`
- 带认证：用户名填写 `admin`，密码填写 `secret`

### Windows 管理服务

| 服务 | 默认端口 | 默认账户 | 说明 |
|------|---------|---------|------|
| WMI | 135 | administrator | 执行 `wmic nic get` 命令 |

### Oracle 服务名自动检测

Oracle 连接支持自动尝试多个常见服务名：
- XE
- ORCL
- XEPDB1
- ORCLPDB
- ORCLCDB
- PDBORCL

---

## ⚙️ 配置说明

### 修改登录密码

编辑 `config.json` 文件：

```json
{
  "password": "your_new_password",
  "port": "18921"
}
```

修改后重启服务生效。

### 修改服务端口

编辑 `config.json` 文件中的 `port` 字段，然后重启服务。

### 数据库文件

- **位置**：程序运行目录下的 `connections.db`
- **格式**：SQLite 3
- **备份**：直接复制 `connections.db` 文件即可

---

## 📁 项目结构

```
批量连接器/
├── main.go                    # 程序入口，路由配置
├── go.mod                     # Go 模块定义
├── go.sum                     # 依赖版本锁定
├── config.json                # 配置文件（密码、端口）
├── connections.db              # SQLite 数据库文件
├── build.sh                   # Linux/macOS 编译脚本
├── build.bat                  # Windows 编译脚本
├── example.csv                # CSV 导入示例文件
├── README.md                  # 项目文档
│
├── internal/                  # 内部包
│   ├── config/               # 配置管理
│   │   └── config.go         # 配置加载和读取
│   │
│   ├── handlers/             # HTTP 处理器
│   │   └── handler.go        # API 路由处理函数
│   │
│   ├── models/               # 数据模型
│   │   └── connection.go     # Connection 结构体定义
│   │
│   └── services/             # 业务逻辑层
│       ├── connector.go      # 连接服务核心逻辑
│       ├── connectors.go     # 各协议连接实现
│       └── database.go       # 数据库操作
│
└── web/                      # Web 前端资源
    ├── templates/            # HTML 模板
    │   ├── index.html        # 主页面
    │   └── login.html        # 登录页面
    │
    └── static/              # 静态资源
        ├── style.css        # 样式文件
        └── script.js        # 前端逻辑
```

### 关键文件说明

- **main.go**: 程序入口，初始化服务、配置路由、启动 HTTP 服务器
- **handlers/handler.go**: 处理所有 HTTP 请求，包括 CSV 导入、连接测试、数据查询等
- **services/connector.go**: 连接管理的核心逻辑，数据库操作
- **services/connectors.go**: 各种协议的连接实现，包含未授权检测逻辑
- **services/database.go**: SQLite 数据库初始化和操作封装

---

## 🔧 交叉编译

项目提供了交叉编译脚本，支持编译到多个平台和架构。

### Linux/macOS 使用 build.sh

```bash
# 编译所有平台
./build.sh

# 编译指定平台
./build.sh -p linux/amd64

# 指定版本号
./build.sh -v 1.1.0

# 清理构建目录
./build.sh -c

# 查看帮助
./build.sh -h
```

### Windows 使用 build.bat

```cmd
# 编译所有平台
build.bat

# 指定版本号（需要先设置环境变量）
set VERSION=1.1.0
build.bat
```

### 支持的平台

- Linux (amd64, arm64)
- Windows (amd64, arm64)
- macOS (amd64, arm64)

编译后的文件会输出到 `build/` 目录，包含：
- 可执行文件
- web 目录（前端资源）
- README.md
- example.csv
- 压缩包（.tar.gz 或 .zip）

---

## 🔐 SSH 命令执行

SSH 连接成功后，工具会自动执行以下命令并显示结果：

- `whoami`: 显示当前登录用户
- `ip addr`: 显示网络接口信息

执行结果会保存在连接的 `result` 字段中，可通过"详情"按钮查看。

---

## ⚠️ 注意事项

### 安全提示

1. **合法使用**：此工具仅用于合法的安全测试和红队演练
2. **授权测试**：请确保您有权限测试目标系统
3. **密码安全**：修改默认密码，妥善保管配置文件
4. **数据安全**：数据库文件包含敏感信息，请妥善保管

### 使用限制

1. **并发连接**：批量连接时会并发执行，注意目标系统负载
2. **超时设置**：默认连接超时为 5 秒，可根据需要调整
3. **Oracle 连接**：使用纯 Go 实现的驱动，无需安装 Oracle Instant Client

### 常见问题

**Q: 为什么 Oracle 连接失败？**  
A: Oracle 连接会自动尝试多个常见服务名，如果都失败，请检查服务名是否正确。

**Q: 如何清除使用须知弹窗？**  
A: 在浏览器控制台执行：`localStorage.removeItem('notice_read')`

**Q: 如何重置登录密码？**  
A: 编辑 `config.json` 文件，修改 `password` 字段后重启服务。

**Q: 数据库文件在哪里？**  
A: 在程序运行目录下的 `connections.db` 文件。

---

## 📝 更新日志

### v1.1.0

- ✅ 新增 Elasticsearch 服务探测，默认请求 `/_cat/indices?pretty`
- ✅ 新增 Zookeeper 探测，默认执行 `ls /` 并支持 digest 认证/多节点
- ✅ 全局 SOCKS5 代理可视化配置，前端即可切换
- ✅ SQLite 切换至纯 Go 驱动（modernc.org/sqlite），无需 CGO
- ✅ 登录/添加连接表单增加服务占位提示（含 ES）

### v1.0.0

- ✅ 支持 12 种服务类型的连接测试
- ✅ CSV 批量导入功能
- ✅ Web 界面管理
- ✅ 未授权访问自动检测
- ✅ SSH 命令自动执行
- ✅ 密码保护功能
- ✅ 使用须知弹窗
- ✅ Oracle 纯 Go 驱动支持（无需 Instant Client）
- ✅ Oracle 服务名自动检测

---

## 📄 许可证

MIT License

---

## 👥 贡献者

**公众号：知攻善防实验室**  
**开发者：ChinaRan404**

---

## 📮 反馈与支持

如有问题或建议，请通过以下方式联系：

- [GitHub Issues](https://github.com/ChinaRan0/Attack_login/issues)
- 公众号：知攻善防实验室

---

**⚠️ 免责声明**：本工具仅供安全研究和合法授权测试使用。使用者需自行承担使用本工具所产生的任何法律责任。
