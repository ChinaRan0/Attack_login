
let currentCategory = '';
let refreshInterval = null; // 自动刷新定时器
const filters = {
    port: '',
    user: '',
    status: '',
    message: ''
};

// 选择分类
function selectCategory(event, type) {
    event.preventDefault();
    currentCategory = type;
    
    // 更新活动状态
    document.querySelectorAll('.category-item').forEach(item => {
        item.classList.remove('active');
    });
    event.currentTarget.classList.add('active');
    
    // 更新标题
    const categoryName = type === '' ? '全部' : type;
    document.getElementById('current-category').textContent = categoryName + '连接';
    
    // 刷新连接列表
    refreshConnections();
}

// 检查响应是否未授权
function checkAuth(response) {
    if (response.status === 401) {
        window.location.href = '/login';
        return true;
    }
    return false;
}

// 安全的 fetch 包装函数，自动处理认证
async function safeFetch(url, options = {}) {
    const response = await fetch(url, options);
    if (checkAuth(response)) {
        return null;
    }
    return response;
}

// 刷新连接列表
async function refreshConnections() {
    let url = '/api/connections';
    const params = new URLSearchParams();
    if (currentCategory) {
        params.append('type', currentCategory);
    }

    if (filters.port) {
        params.append('port', filters.port);
    }
    if (filters.user) {
        params.append('user', filters.user);
    }
    if (filters.status) {
        params.append('status', filters.status);
    }
    if (filters.message) {
        params.append('message', filters.message);
    }

    const queryString = params.toString();
    if (queryString) {
        url += '?' + queryString;
    }

    try {
        const response = await safeFetch(url);
        if (!response) return;
        const data = await response.json();
        displayConnections(data.connections);
        updateCategoryCounts(data.connections);
        
        // 检查是否所有连接任务都已完成
        checkAndStopAutoRefresh(data.connections);
    } catch (error) {
        console.error('刷新连接列表失败:', error);
    }
}

// 应用筛选
function applyFilters(event) {
    if (event) {
        event.preventDefault();
    }
    filters.port = document.getElementById('filter-port').value.trim();
    filters.user = document.getElementById('filter-user').value.trim();
    filters.status = document.getElementById('filter-status').value;
    filters.message = document.getElementById('filter-message').value.trim();
    refreshConnections();
}

// 重置筛选
function resetFilters() {
    const form = document.getElementById('filters-form');
    if (form) {
        form.reset();
    }
    filters.port = '';
    filters.user = '';
    filters.status = '';
    filters.message = '';
    refreshConnections();
}

// 检查并停止自动刷新
function checkAndStopAutoRefresh(connections) {
    if (!connections || connections.length === 0) {
        // 没有连接，停止自动刷新
        stopAutoRefresh();
        return;
    }
    
    // 检查是否有 pending 状态的连接
    const hasPending = connections.some(conn => conn.status === 'pending');
    
    if (!hasPending && refreshInterval) {
        // 所有连接都已完成，停止自动刷新
        stopAutoRefresh();
        console.log('所有连接任务已完成，已停止自动刷新');
    }
}

// 停止自动刷新
function stopAutoRefresh() {
    if (refreshInterval) {
        clearInterval(refreshInterval);
        refreshInterval = null;
    }
}

// 启动自动刷新
function startAutoRefresh() {
    // 如果已经在运行，先停止
    stopAutoRefresh();
    // 启动新的定时器
    refreshInterval = setInterval(refreshConnections, 3000);
}

// 更新分类计数
function updateCategoryCounts(connections) {
    const counts = {
        'all': connections.length,
        'Redis': 0,
        'FTP': 0,
        'PostgreSQL': 0,
        'MySQL': 0,
        'SQLServer': 0,
        'RabbitMQ': 0,
        'SSH': 0,
        'MongoDB': 0,
        'SMB': 0,
        'WMI': 0,
        'MQTT': 0,
        'Oracle': 0
    };
    
    connections.forEach(conn => {
        if (counts.hasOwnProperty(conn.type)) {
            counts[conn.type]++;
        }
    });
    
    document.getElementById('count-all').textContent = counts.all;
    document.getElementById('count-Redis').textContent = counts.Redis;
    document.getElementById('count-FTP').textContent = counts.FTP;
    document.getElementById('count-PostgreSQL').textContent = counts.PostgreSQL;
    document.getElementById('count-MySQL').textContent = counts.MySQL;
    document.getElementById('count-SQLServer').textContent = counts.SQLServer;
    document.getElementById('count-RabbitMQ').textContent = counts.RabbitMQ;
    document.getElementById('count-SSH').textContent = counts.SSH;
    document.getElementById('count-MongoDB').textContent = counts.MongoDB;
    document.getElementById('count-SMB').textContent = counts.SMB;
    document.getElementById('count-WMI').textContent = counts.WMI;
    document.getElementById('count-MQTT').textContent = counts.MQTT;
    document.getElementById('count-Oracle').textContent = counts.Oracle;
}

// 显示连接列表
function displayConnections(connections) {
    const listDiv = document.getElementById('connections-list');
    
    if (!connections || connections.length === 0) {
        listDiv.innerHTML = '<div class="empty-state"><h3>暂无连接记录</h3><p>请先导入 CSV 文件或手动添加连接</p></div>';
        return;
    }

    let html = '<table class="connections-table">';
    html += '<thead><tr>';
    html += '<th style="width: 30px;"><input type="checkbox" id="select-all" onchange="toggleSelectAll()"></th>';
    html += '<th style="width: 100px;">类型</th>';
    html += '<th style="width: 150px;">IP</th>';
    html += '<th style="width: 80px;">端口</th>';
    html += '<th style="width: 100px;">用户</th>';
    html += '<th style="width: 100px;">密码</th>';
    html += '<th style="width: 100px;">状态</th>';
    html += '<th>消息</th>';
    html += '<th style="width: 150px;">创建时间</th>';
    html += '<th style="width: 250px;">操作</th>';
    html += '</tr></thead>';
    html += '<tbody>';
    
    connections.forEach(conn => {
        html += createConnectionRow(conn);
    });
    
    html += '</tbody></table>';
    listDiv.innerHTML = html;
}

// 创建连接表格行
function createConnectionRow(conn) {
    const statusClass = conn.status === 'success' ? 'success' : 
                       conn.status === 'failed' ? 'failed' : 'pending';
    const statusText = conn.status === 'success' ? '成功' : 
                      conn.status === 'failed' ? '失败' : '连接中';
    
    const typeClass = conn.type.toLowerCase();
    const date = new Date(conn.created_at).toLocaleString('zh-CN');
    
    // 检查是否有日志或结果需要展开显示
    const hasDetails = (conn.logs && conn.logs.length > 0) || conn.result;
    const rowId = `row-${conn.id}`;
    const detailsId = `details-${conn.id}`;

    let detailsHtml = '';
    if (hasDetails) {
        let logsHtml = '';
        if (conn.logs && conn.logs.length > 0) {
            logsHtml = '<div class="connection-logs"><strong>连接日志:</strong><ul>';
            conn.logs.forEach(log => {
                logsHtml += `<li>${escapeHtml(log)}</li>`;
            });
            logsHtml += '</ul></div>';
        }

        let resultHtml = '';
        if (conn.result) {
            resultHtml = `<div class="connection-result"><strong>命令执行结果:</strong><br>${escapeHtml(conn.result)}</div>`;
        }

        detailsHtml = `
            <tr id="${detailsId}" class="connection-details-row" style="display: none;">
                <td colspan="10">
                    <div class="connection-details">
                        ${logsHtml}
                        ${resultHtml}
                    </div>
                </td>
            </tr>
        `;
    }

    return `
        <tr id="${rowId}" class="connection-row">
            <td>
                <input type="checkbox" class="conn-checkbox" value="${conn.id}">
            </td>
            <td>
                <span class="connection-type ${typeClass}">${escapeHtml(conn.type)}</span>
            </td>
            <td>${escapeHtml(conn.ip)}</td>
            <td>${escapeHtml(conn.port)}</td>
            <td>${conn.user ? escapeHtml(conn.user) : '-'}</td>
            <td>${conn.pass ? escapeHtml(conn.pass) : '-'}</td>
            <td>
                <span class="connection-status ${statusClass}">${statusText}</span>
            </td>
            <td class="message-cell" title="${escapeHtml(conn.message || '无')}">
                ${escapeHtml((conn.message || '无').substring(0, 50))}${(conn.message || '').length > 50 ? '...' : ''}
            </td>
            <td style="font-size: 12px; color: #999;">${date}</td>
            <td>
                <div class="table-actions">
                    ${hasDetails ? `<button class="btn btn-sm btn-secondary" onclick="toggleDetails('${conn.id}')">详情</button>` : ''}
                    <button class="btn btn-sm btn-primary" onclick="editConnection('${conn.id}')">编辑</button>
                    <button class="btn btn-sm btn-success" onclick="connectSingle('${conn.id}')">重连</button>
                    <button class="btn btn-sm btn-danger" onclick="deleteConnection('${conn.id}')">删除</button>
                </div>
            </td>
        </tr>
        ${detailsHtml}
    `;
}

// 切换详情显示
function toggleDetails(id) {
    const detailsRow = document.getElementById(`details-${id}`);
    if (detailsRow) {
        if (detailsRow.style.display === 'none') {
            detailsRow.style.display = 'table-row';
        } else {
            detailsRow.style.display = 'none';
        }
    }
}

// 全选/取消全选
function toggleSelectAll() {
    const selectAll = document.getElementById('select-all');
    const checkboxes = document.querySelectorAll('.conn-checkbox');
    checkboxes.forEach(cb => {
        cb.checked = selectAll.checked;
    });
}

// 转义 HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 显示导入模态框
function showImportModal() {
    document.getElementById('import-modal').classList.add('active');
}

// 服务类型配置（默认端口和默认账户）
const serviceConfig = {
    'Redis': { port: '6379', user: '留空表示未授权访问（无密码）' },
    'FTP': { port: '21', user: '留空表示匿名登录（anonymous/anonymous）' },
    'PostgreSQL': { port: '5432', user: '留空表示默认用户 postgres' },
    'MySQL': { port: '3306', user: '留空表示默认用户 root' },
    'SQLServer': { port: '1433', user: '留空表示默认用户 sa' },
    'RabbitMQ': { port: '5672', user: '留空表示默认用户 guest/guest' },
    'SSH': { port: '22', user: '留空表示默认用户 root 或 admin' },
    'MongoDB': { port: '27017', user: '留空表示未授权访问（无认证）' },
    'SMB': { port: '445', user: '留空表示默认用户 administrator' },
    'WMI': { port: '135', user: '留空表示默认用户 administrator' },
    'MQTT': { port: '1883', user: '留空表示默认用户 admin/admin' },
    'Oracle': { port: '1521', user: '留空表示默认用户 sys/system 或 scott/tiger' }
};

// 更新端口和用户名的 placeholder
function updateConnectionFormPlaceholders(type) {
    const portInput = document.getElementById('conn-port');
    const userInput = document.getElementById('conn-user');
    
    if (type && serviceConfig[type]) {
        const config = serviceConfig[type];
        if (portInput) {
            portInput.placeholder = config.port;
        }
        if (userInput) {
            userInput.placeholder = config.user;
        }
    } else {
        // 默认值
        if (portInput) {
            portInput.placeholder = '3306';
        }
        if (userInput) {
            userInput.placeholder = '留空表示未授权访问';
        }
    }
}

// 更新编辑表单的 placeholder
function updateEditFormPlaceholders(type) {
    const portInput = document.getElementById('edit-port');
    const userInput = document.getElementById('edit-user');
    
    if (type && serviceConfig[type]) {
        const config = serviceConfig[type];
        if (portInput) {
            portInput.placeholder = config.port;
        }
        if (userInput) {
            userInput.placeholder = config.user;
        }
    } else {
        // 默认值
        if (portInput) {
            portInput.placeholder = '3306';
        }
        if (userInput) {
            userInput.placeholder = '留空表示未授权访问';
        }
    }
}

// 显示添加模态框
function showAddModal() {
    document.getElementById('add-modal').classList.add('active');
    // 重置表单
    const typeSelect = document.getElementById('conn-type');
    if (typeSelect) {
        typeSelect.value = '';
        updateConnectionFormPlaceholders('');
    }
}

// 关闭模态框
function closeModal(modalId) {
    document.getElementById(modalId).classList.remove('active');
}

// CSV 导入
document.getElementById('import-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const formData = new FormData();
    const fileInput = document.getElementById('csv-file');
    
    if (!fileInput.files[0]) {
        showResult('import-result', '请选择文件', 'error');
        return;
    }

    formData.append('file', fileInput.files[0]);

    const resultDiv = document.getElementById('import-result');
    resultDiv.className = 'result';
    resultDiv.textContent = '导入中...';

    try {
        const response = await safeFetch('/api/import', {
            method: 'POST',
            body: formData
        });
        if (!response) return;

        const data = await response.json();
        if (response.ok) {
            showResult('import-result', `成功导入 ${data.count} 条连接记录`, 'success');
            fileInput.value = '';
            closeModal('import-modal');
            startAutoRefresh(); // 启动自动刷新
            setTimeout(refreshConnections, 500);
        } else {
            showResult('import-result', '导入失败: ' + data.error, 'error');
        }
    } catch (error) {
        showResult('import-result', '导入失败: ' + error.message, 'error');
    }
});

// 手动添加连接
document.getElementById('manual-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const formData = {
        type: document.getElementById('conn-type').value,
        ip: document.getElementById('conn-ip').value,
        port: document.getElementById('conn-port').value,
        user: document.getElementById('conn-user').value,
        pass: document.getElementById('conn-pass').value
    };

    const resultDiv = document.getElementById('manual-result');
    resultDiv.className = 'result';
    resultDiv.textContent = '连接中...';

    try {
        const response = await safeFetch('/api/connect', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(formData)
        });
        if (!response) return;

        const data = await response.json();
        if (response.ok) {
            showResult('manual-result', '连接任务已启动', 'success');
            document.getElementById('manual-form').reset();
            closeModal('add-modal');
            startAutoRefresh(); // 启动自动刷新
            setTimeout(refreshConnections, 500);
        } else {
            showResult('manual-result', '连接失败: ' + data.error, 'error');
        }
    } catch (error) {
        showResult('manual-result', '连接失败: ' + error.message, 'error');
    }
});

// 显示结果
function showResult(elementId, message, type) {
    const resultDiv = document.getElementById(elementId);
    resultDiv.textContent = message;
    resultDiv.className = `result ${type}`;
}

// 单个连接
async function connectSingle(id) {
    try {
        const response = await safeFetch('/api/connect', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                id: id
            })
        });
        if (!response) return;

        const data = await response.json();
        if (response.ok) {
            alert('连接任务已启动');
            startAutoRefresh(); // 启动自动刷新
            setTimeout(refreshConnections, 2000);
        } else {
            alert('连接失败: ' + (data.error || '未知错误'));
        }
    } catch (error) {
        alert('连接失败: ' + error.message);
    }
}

// 批量连接
async function connectAll() {
    const ids = getSelectedIds();
    if (ids.length === 0) {
        alert('请先选择要连接的记录');
        return;
    }

    try {
        const response = await safeFetch('/api/connect-batch', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ ids: ids })
        });
        if (!response) return;

        const data = await response.json();
        if (response.ok) {
            alert(`已启动 ${data.count} 个连接任务`);
            startAutoRefresh(); // 启动自动刷新
            setTimeout(refreshConnections, 2000);
        } else {
            alert('批量连接失败: ' + data.error);
        }
    } catch (error) {
        alert('批量连接失败: ' + error.message);
    }
}

// 批量删除连接
async function deleteSelected() {
    const ids = getSelectedIds();
    if (ids.length === 0) {
        alert('请先选择要删除的记录');
        return;
    }

    if (!confirm(`确认删除选中的 ${ids.length} 条记录吗？`)) {
        return;
    }

    try {
        const response = await safeFetch('/api/connections/delete-batch', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ ids })
        });
        if (!response) return;

        const data = await response.json();
        if (response.ok) {
            alert(data.message || `已删除 ${data.count} 条记录`);
            refreshConnections();
        } else {
            alert('批量删除失败: ' + (data.error || '未知错误'));
        }
    } catch (error) {
        alert('批量删除失败: ' + error.message);
    }
}

function getSelectedIds() {
    const checkboxes = document.querySelectorAll('.conn-checkbox:checked');
    return Array.from(checkboxes).map(cb => cb.value);
}

// 编辑连接
let currentEditId = null;

function editConnection(id) {
    currentEditId = id;
    
    // 获取连接信息
    fetch('/api/connections')
        .then(response => response.json())
        .then(data => {
            const conn = data.connections.find(c => c.id === id);
            if (!conn) {
                alert('连接不存在');
                return;
            }
            
            // 填充表单
            const type = conn.type || '';
            document.getElementById('edit-type').value = type;
            document.getElementById('edit-ip').value = conn.ip || '';
            document.getElementById('edit-port').value = conn.port || '';
            document.getElementById('edit-user').value = conn.user || '';
            document.getElementById('edit-pass').value = ''; // 不显示密码
            
            // 更新 placeholder
            updateEditFormPlaceholders(type);
            
            // 显示模态框
            document.getElementById('edit-modal').classList.add('active');
        })
        .catch(error => {
            console.error('获取连接信息失败:', error);
            alert('获取连接信息失败: ' + error.message);
        });
}

// 编辑表单提交
document.getElementById('edit-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    
    if (!currentEditId) {
        alert('无效的连接 ID');
        return;
    }
    
    const formData = {
        type: document.getElementById('edit-type').value,
        ip: document.getElementById('edit-ip').value,
        port: document.getElementById('edit-port').value,
        user: document.getElementById('edit-user').value,
        pass: document.getElementById('edit-pass').value
    };

    const resultDiv = document.getElementById('edit-result');
    resultDiv.className = 'result';
    resultDiv.textContent = '更新中...';

    try {
        const response = await safeFetch(`/api/connections/${currentEditId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(formData)
        });
        if (!response) return;

        const data = await response.json();
        if (response.ok) {
            showResult('edit-result', '连接更新成功', 'success');
            document.getElementById('edit-form').reset();
            closeModal('edit-modal');
            currentEditId = null;
            setTimeout(refreshConnections, 500);
        } else {
            showResult('edit-result', '更新失败: ' + data.error, 'error');
        }
    } catch (error) {
        showResult('edit-result', '更新失败: ' + error.message, 'error');
    }
});

// 删除连接
async function deleteConnection(id) {
    if (!confirm('确定要删除这条连接记录吗？')) {
        return;
    }

    try {
        const response = await safeFetch(`/api/connections/${id}`, {
            method: 'DELETE'
        });
        if (!response) return;

        if (response.ok) {
            refreshConnections();
        } else {
            alert('删除失败');
        }
    } catch (error) {
        alert('删除失败: ' + error.message);
    }
}

// 点击模态框外部关闭
document.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal')) {
        e.target.classList.remove('active');
    }
});

// 登出
async function logout() {
    try {
        const response = await fetch('/api/logout', {
            method: 'POST'
        });
        if (response.ok) {
            window.location.href = '/login';
        }
    } catch (error) {
        console.error('登出失败:', error);
        window.location.href = '/login';
    }
}

// 打开代理设置
async function openProxySettings() {
    const modal = document.getElementById('proxy-modal');
    if (!modal) {
        return;
    }
    resetProxyResult();
    try {
        const response = await safeFetch('/api/settings/proxy');
        if (!response) return;
        const data = await response.json();
        populateProxyForm(data.proxy || {});
        modal.classList.add('active');
    } catch (error) {
        alert('获取代理配置失败: ' + error.message);
    }
}

function populateProxyForm(proxy) {
    const enabledInput = document.getElementById('proxy-enabled');
    const hostInput = document.getElementById('proxy-host');
    const portInput = document.getElementById('proxy-port');
    const userInput = document.getElementById('proxy-user');
    const passInput = document.getElementById('proxy-pass');

    if (enabledInput) {
        enabledInput.checked = Boolean(proxy.enabled);
    }
    if (hostInput) {
        hostInput.value = proxy.host || '';
    }
    if (portInput) {
        portInput.value = proxy.port || '';
    }
    if (userInput) {
        userInput.value = proxy.user || '';
    }
    if (passInput) {
        passInput.value = proxy.pass || '';
    }
    updateProxyFieldsState();
}

function updateProxyFieldsState() {
    const enabledInput = document.getElementById('proxy-enabled');
    const enabled = enabledInput ? enabledInput.checked : false;
    const fields = document.querySelectorAll('.proxy-field');
    fields.forEach(field => {
        field.disabled = !enabled;
        if (!enabled) {
            field.classList.add('input-disabled');
        } else {
            field.classList.remove('input-disabled');
        }
    });
}

function resetProxyResult() {
    const resultDiv = document.getElementById('proxy-result');
    if (resultDiv) {
        resultDiv.className = 'result';
        resultDiv.textContent = '';
    }
}

async function submitProxySettings(event) {
    event.preventDefault();
    const enabledInput = document.getElementById('proxy-enabled');
    const hostInput = document.getElementById('proxy-host');
    const portInput = document.getElementById('proxy-port');
    const userInput = document.getElementById('proxy-user');
    const passInput = document.getElementById('proxy-pass');
    const resultDivId = 'proxy-result';

    const payload = {
        enabled: enabledInput ? enabledInput.checked : false,
        type: 'socks5',
        host: hostInput ? hostInput.value.trim() : '',
        port: portInput ? portInput.value.trim() : '',
        user: userInput ? userInput.value.trim() : '',
        pass: passInput ? passInput.value : ''
    };

    if (payload.enabled && (!payload.host || !payload.port)) {
        showResult(resultDivId, '启用代理时必须填写主机和端口', 'error');
        return;
    }

    resetProxyResult();
    const resultDiv = document.getElementById(resultDivId);
    if (resultDiv) {
        resultDiv.textContent = '保存中...';
        resultDiv.className = 'result';
        resultDiv.style.display = 'block';
    }

    try {
        const response = await safeFetch('/api/settings/proxy', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });
        if (!response) return;
        const data = await response.json();
        if (response.ok) {
            showResult(resultDivId, data.message || '代理配置已更新', 'success');
        } else {
            showResult(resultDivId, data.error || '代理配置保存失败', 'error');
        }
    } catch (error) {
        showResult(resultDivId, '代理配置保存失败: ' + error.message, 'error');
    }
}

// 显示使用须知弹窗
function showNoticeModal() {
    const modal = document.getElementById('notice-modal');
    if (modal) {
        modal.style.display = 'flex';
        console.log('使用须知弹窗已显示');
    } else {
        console.error('未找到弹窗元素 notice-modal');
    }
}

// 关闭使用须知弹窗
function closeNoticeModal() {
    const modal = document.getElementById('notice-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// 处理弹窗点击事件（点击外部关闭）
function handleNoticeModalClick(event) {
    if (event.target.id === 'notice-modal') {
        closeNoticeModal();
    }
}

// 确认使用须知
function confirmNotice() {
    const checkbox = document.getElementById('notice-checkbox');
    if (checkbox && checkbox.checked) {
        // 保存到 localStorage，标记已阅读
        localStorage.setItem('notice_read', 'true');
        closeNoticeModal();
    } else {
        // 提示用户勾选复选框
        alert('请勾选"我已阅读并同意上述使用须知"复选框');
    }
}

// 检查是否需要显示使用须知
function checkNotice() {
    const noticeRead = localStorage.getItem('notice_read');
    console.log('检查使用须知状态:', noticeRead);
    if (!noticeRead || noticeRead !== 'true') {
        // 首次访问，显示使用须知
        console.log('准备显示使用须知弹窗');
        setTimeout(() => {
            const modal = document.getElementById('notice-modal');
            if (modal) {
                console.log('找到弹窗元素，准备显示');
                modal.style.display = 'flex';
            } else {
                console.error('未找到弹窗元素 notice-modal');
            }
        }, 500); // 延迟500ms显示，让页面先加载完成
    } else {
        console.log('用户已阅读使用须知，不显示弹窗');
    }
}

// 页面加载时刷新连接列表
document.addEventListener('DOMContentLoaded', () => {
    refreshConnections();
    // 启动自动刷新（如果所有任务都已完成，会自动停止）
    startAutoRefresh();
    // 检查是否需要显示使用须知（延迟一点确保 DOM 完全加载）
    setTimeout(() => {
        checkNotice();
    }, 100);

    const proxyForm = document.getElementById('proxy-form');
    if (proxyForm) {
        proxyForm.addEventListener('submit', submitProxySettings);
    }
    const proxyToggle = document.getElementById('proxy-enabled');
    if (proxyToggle) {
        proxyToggle.addEventListener('change', updateProxyFieldsState);
    }
});
