/**
 * WiFi Captive Portal SaaS 运营后台客户端核心逻辑
 * - 包含完整的登录态维护、角色鉴权适配
 * - 升级为全新水平横向顶部导航大盘 UI 风格
 * - 增设超级管理员动态加权短信通道池 (SMS Providers) 全功能 CRUD
 * - 模态框与动态数据渲染、高拟真视觉动效与友好提示
 */

// 全局状态管理
const state = {
    user: null,
    level: 0,
    activeTab: 'dashboard',
    isEditingHotel: false,
    isEditingPackage: false,
    isEditingSMSProvider: false,
    hotels: [],
    packages: [],
    smsProviders: [],
    
    // 列表搜索、过滤与分页状态
    hotelsPage: 1,
    hotelsSearch: '',
    
    auditLogs: [],
    auditPage: 1,
    auditHotelId: '',
    auditSearch: '',
    
    smsLogs: [],
    smsPage: 1,
    smsHotelId: '',
    
    rechargeLogs: [],
    rechargePage: 1
};

// 页面加载初始化
document.addEventListener('DOMContentLoaded', () => {
    initApp();
});

// 初始化应用
async function initApp() {
    setupTabNavigation();
    setupEventListeners();
    
    // 动态绘制设备对接指南示例地址，匹配当前域名/IP与端口
    const guideUrlEl = document.getElementById('portal-config-url-example');
    if (guideUrlEl) {
        guideUrlEl.innerText = `${window.location.origin}/hotel/[您的HotelId]`;
    }
    
    await checkLoginStatus();
}

// =========================================================================
// 1. 登录态与权限管理
// =========================================================================

// 检查登录状态
async function checkLoginStatus() {
    try {
        const res = await fetch('/api/admin/profile');
        if (res.ok) {
            const profile = await res.json();
            state.user = profile.user;
            state.level = profile.level;
            
            // 更新顶部钱包及用户信息
            updateWalletUI(profile.sms_count, profile.balance);
            document.getElementById('current-user-name').innerText = `账号: ${profile.user}`;
            
            const levelBadge = document.getElementById('current-user-level');
            if (profile.level >= 50) {
                levelBadge.innerText = '系统超管';
                levelBadge.className = 'badge badge-admin';
                // 显示超管独占菜单与按钮
                document.getElementById('admin-divider').style.display = 'inline-block';
                document.querySelectorAll('.admin-only').forEach(el => el.style.display = 'inline-block');
                document.querySelectorAll('.admin-only-inline').forEach(el => el.style.display = 'inline-block');
            } else {
                levelBadge.innerText = '酒店商户';
                levelBadge.className = 'badge';
                // 隐藏超管独占菜单与按钮
                document.getElementById('admin-divider').style.display = 'none';
                document.querySelectorAll('.admin-only').forEach(el => el.style.display = 'none');
                document.querySelectorAll('.admin-only-inline').forEach(el => el.style.display = 'none');
            }

            // 隐藏登录框，展现主界面
            document.getElementById('login-container').style.display = 'none';
            document.getElementById('app-container').style.display = 'flex';
            
            // 默认加载第一个 tab
            switchTab(state.activeTab);
        } else {
            showLoginOverlay();
        }
    } catch (err) {
        console.error('检查登录态错误:', err);
        showLoginOverlay();
    }
}

// 显示登录遮罩
function showLoginOverlay() {
    document.getElementById('login-container').style.display = 'flex';
    document.getElementById('app-container').style.display = 'none';
}

// 更新头部钱包余额及短信条数
function updateWalletUI(smsCount, balanceCents) {
    document.getElementById('wallet-sms').innerText = `${smsCount} 条`;
    document.getElementById('wallet-balance').innerText = `${(balanceCents / 100).toFixed(2)} 元`;
}

// =========================================================================
// 2. 界面视图选项卡切换 (Tab Routing)
// =========================================================================

function setupTabNavigation() {
    const navItems = document.querySelectorAll('.nav-item');
    navItems.forEach(item => {
        item.addEventListener('click', () => {
            const tabName = item.getAttribute('data-tab');
            switchTab(tabName);
        });
    });
}

function switchTab(tabName) {
    state.activeTab = tabName;
    
    // 更新导航高亮
    document.querySelectorAll('.nav-item').forEach(item => {
        if (item.getAttribute('data-tab') === tabName) {
            item.classList.add('active');
        } else {
            item.classList.remove('active');
        }
    });

    // 更新面板显示
    document.querySelectorAll('.tab-pane').forEach(pane => {
        if (pane.id === `pane-${tabName}`) {
            pane.classList.add('active');
        } else {
            pane.classList.remove('active');
        }
    });

    // 更新头部大标题及副标题说明
    const titles = {
        'dashboard': '📊 业务大屏',
        'hotels': '🏨 酒店及网关管理',
        'order-packages': '🛒 订购短信包商城',
        'audit-logs': '📝 审计日志',
        'sms-logs': '💬 短信明细',
        'recharge-logs': '💳 账单记录',
        'super-users': '👥 系统商户及钱包划拨中心',
        'super-packages': '⚙️ 运营短信套餐池维护',
        'super-sms-providers': '⚙️ 短信通道及负载配置'
    };
    
    const subtitles = {
        'dashboard': '系统全局运行状态概览，实时汇总关键网络运行指标',
        'hotels': '管理所有注册的酒店节点，支持一键配置接口对接参数',
        'order-packages': '自主订购，余额原子扣减，秒级生效，价格极其优惠',
        'audit-logs': '符合公共安全联网审计标准，记录终端 MAC 手机号信息（默认最近 100 条）',
        'sms-logs': '账单完全透明，支持查询每一笔短信费用流向',
        'recharge-logs': '系统自动生成高可信 ULID 订单号，支持历史订单对账查询',
        'super-users': '系统超级管理员专属模块，支持全局资金划拨与套餐赠送',
        'super-packages': '自由配置多档短信套餐包的价格及对应条数，调整后全网秒级同步上架',
        'super-sms-providers': '多通道加权分流路由高可用，超级管理员专享短信节点配置'
    };
    
    document.getElementById('tab-title').innerText = titles[tabName] || '运营后台';
    document.getElementById('tab-subtitle').innerText = subtitles[tabName] || '多节点负载调度大盘';

    // 触发数据加载
    loadTabData(tabName);
}

// 统一数据加载路由器
async function loadTabData(tabName) {
    switch (tabName) {
        case 'dashboard':
            await loadDashboardStats();
            break;
        case 'hotels':
            await loadHotels();
            break;
        case 'order-packages':
            await loadPackagesShop();
            break;
        case 'audit-logs':
            await loadAuditLogs();
            break;
        case 'sms-logs':
            await loadSMSLogs();
            break;
        case 'recharge-logs':
            await loadRechargeLogs();
            break;
        case 'super-users':
            await loadSuperUsers();
            break;
        case 'super-packages':
            await loadSuperPackages();
            break;
        case 'super-sms-providers':
            await loadSuperSMSProviders();
            break;
    }
}

// =========================================================================
// 3. 各模块数据拉取与渲染
// =========================================================================

// 3.1 业务大屏统计
async function loadDashboardStats() {
    try {
        // 重新拉取一次 profile 刷新顶部状态
        const profileRes = await fetch('/api/admin/profile');
        if (profileRes.ok) {
            const profile = await profileRes.json();
            updateWalletUI(profile.sms_count, profile.balance);
        }

        // 加载酒店数据以更新酒店数
        let hotelCount = 0;
        if (state.level >= 50) {
            const res = await fetch('/api/admin/hotels');
            if (res.ok) {
                const list = await res.json();
                hotelCount = list.length;
            }
        } else {
            const res = await fetch('/api/merchant/hotels');
            if (res.ok) {
                const list = await res.json();
                hotelCount = list.length;
            }
        }
        document.getElementById('stat-hotels').innerText = `${hotelCount} 个`;

        // 加载审计日志与短信日志获取连接次数和发送量
        let authCount = 0;
        let smsCount = 0;
        
        // 只有商户有审计/短信日志接口
        const authRes = await fetch('/api/merchant/auth-logs');
        if (authRes.ok) {
            const list = await authRes.json();
            authCount = list.length;
        }

        const smsRes = await fetch('/api/merchant/sms-logs');
        if (smsRes.ok) {
            const list = await smsRes.json();
            smsCount = list.length;
        }

        document.getElementById('stat-auths').innerText = `${authCount} 次`;
        document.getElementById('stat-sms').innerText = `${smsCount} 次`;

    } catch (err) {
        console.error('加载大屏统计失败:', err);
    }
}

// 3.2 酒店列表
async function loadHotels() {
    const tbody = document.getElementById('hotels-table-body');
    tbody.innerHTML = '<tr><td colspan="10" class="text-center">🔄 正在加载酒店列表...</td></tr>';
    
    try {
        const url = state.level >= 50 ? '/api/admin/hotels' : '/api/merchant/hotels';
        const res = await fetch(url);
        if (!res.ok) throw new Error('拉取酒店列表失败');
        
        state.hotels = await res.json();
        
        // 动态更新审计日志和短信账单明细中的酒店筛选下拉菜单
        updateHotelFilters();
        
        // 渲染酒店列表第一页
        state.hotelsPage = 1;
        renderHotels();
    } catch (err) {
        showToast(err.message, 'danger');
        tbody.innerHTML = '<tr><td colspan="10" class="text-center text-danger">⚠️ 加载数据失败</td></tr>';
    }
}

// 渲染酒店数据表格
function renderHotels() {
    const tbody = document.getElementById('hotels-table-body');
    
    // 1. 关键字搜索过滤 (匹配酒店名称或ID)
    const searchVal = state.hotelsSearch.toLowerCase().trim();
    const filtered = state.hotels.filter(h => {
        if (!searchVal) return true;
        const hotelIdStr = String(h.hotelId);
        const name = String(h.name).toLowerCase();
        return hotelIdStr.includes(searchVal) || name.includes(searchVal);
    });
    
    if (filtered.length === 0) {
        tbody.innerHTML = '<tr><td colspan="10" class="text-center">ℹ️ 暂无符合条件的酒店配置，请重新搜索或新建。</td></tr>';
        updatePaginationBar('hotels-pagination', 0, 1, 15, (page) => {
            state.hotelsPage = page;
            renderHotels();
        });
        return;
    }
    
    // 2. 酒店分页逻辑 (默认一页15个)
    const pageSize = 15;
    const totalCount = filtered.length;
    const totalPages = Math.ceil(totalCount / pageSize);
    
    if (state.hotelsPage > totalPages) state.hotelsPage = totalPages;
    if (state.hotelsPage < 1) state.hotelsPage = 1;
    
    const startIndex = (state.hotelsPage - 1) * pageSize;
    const endIndex = Math.min(startIndex + pageSize, totalCount);
    const pageItems = filtered.slice(startIndex, endIndex);
    
    tbody.innerHTML = '';
    pageItems.forEach(h => {
        const tr = document.createElement('tr');
        
        const driverBadge = {
            'ikuai': '<span class="badge" style="background: rgba(9, 132, 227, 0.08); border-color: rgba(9, 132, 227, 0.25); color: #0984e3;">爱快 IKuai</span>',
            'panabit': '<span class="badge" style="background: rgba(108, 92, 203, 0.08); border-color: rgba(108, 92, 203, 0.25); color: #6c5ce7;">Panabit</span>',
            'mikrotik': '<span class="badge" style="background: rgba(225, 112, 85, 0.08); border-color: rgba(225, 112, 85, 0.25); color: #e17055;">MikroTik</span>'
        }[h.gateway_type] || `<span class="badge">${h.gateway_type}</span>`;

        const bypassBadge = h.bypass_auth === 1 
            ? '<span class="badge" style="background: rgba(16, 185, 129, 0.08); border-color: rgba(16, 185, 129, 0.25); color: var(--accent-glow);">已启用 (30天)</span>'
            : '<span class="badge" style="background: rgba(100, 116, 139, 0.08); border-color: rgba(100, 116, 139, 0.25); color: var(--text-muted);">已禁用</span>';
        
        tr.innerHTML = `
            <td><strong>${h.hotelId}</strong></td>
            <td><strong>${escapeHTML(h.name)}</strong></td>
            <td>
                <button class="btn-sm" style="border-color: var(--primary-accent); color: var(--primary-accent); cursor: pointer;" onclick="copyPortalUrl(${h.hotelId})" title="点击复制对接 Portal URL">📋 对接地址</button>
            </td>
            <td>${driverBadge}</td>
            <td><code class="code-highlight">${escapeHTML(h.custom_name || '-')}</code></td>
            <td>${bypassBadge}</td>
            <td class="text-truncate" style="max-width: 160px;" title="${escapeHTML(h.welcome_text)}">${escapeHTML(h.welcome_text)}</td>
            <td>${h.user}</td>
            <td>${h.status === 1 ? '<span class="badge" style="background: rgba(16, 185, 129, 0.08); border-color: rgba(16, 185, 129, 0.25); color: var(--accent-glow);">● 启用</span>' : '<span class="badge" style="background: rgba(239, 68, 68, 0.08); border-color: rgba(239, 68, 68, 0.25); color: var(--error-glow);">● 禁用</span>'}</td>
            <td>
                <button class="btn-sm" onclick="openEditHotelModal(${h.hotelId})">⚙️ 编辑配置</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
    
    // 更新分页控件条
    updatePaginationBar('hotels-pagination', totalCount, state.hotelsPage, pageSize, (page) => {
        state.hotelsPage = page;
        renderHotels();
    });
}

// 3.3 订购短信包商城
async function loadPackagesShop() {
    const grid = document.getElementById('packages-shop-list');
    grid.innerHTML = '<div class="text-center" style="grid-column: 1/-1;">🔄 正在加载商城货架...</div>';
    
    try {
        const res = await fetch('/api/admin/packages');
        if (!res.ok) throw new Error('拉取套餐包失败');
        
        const list = await res.json();
        const activeList = list.filter(p => p.status === 1);
        
        if (activeList.length === 0) {
            grid.innerHTML = '<div class="text-center" style="grid-column: 1/-1;">ℹ️ 商城暂无上架商品。</div>';
            return;
        }
        
        grid.innerHTML = '';
        activeList.forEach(p => {
            const card = document.createElement('div');
            card.className = 'package-card';
            card.innerHTML = `
                <div class="card-icon">⚡</div>
                <h4>${escapeHTML(p.name)}</h4>
                <div class="sms-amount">${p.sms_count} <span>条短信</span></div>
                <div class="price">售价: <strong>${(p.price / 100).toFixed(2)}</strong> 元</div>
                <p class="unit-price">折合 ${(p.price / p.sms_count).toFixed(2)} 分/条</p>
                <button class="btn-primary btn-block" onclick="buyPackage('${p.packageId}')">立即订购</button>
            `;
            grid.appendChild(card);
        });
    } catch (err) {
        grid.innerHTML = `<div class="text-center text-danger" style="grid-column: 1/-1;">⚠️ ${err.message}</div>`;
    }
}

// 3.4 连网放行审计日志
async function loadAuditLogs() {
    const tbody = document.getElementById('audit-logs-table-body');
    tbody.innerHTML = '<tr><td colspan="6" class="text-center">🔄 正在加载审计明细...</td></tr>';
    
    try {
        const res = await fetch('/api/merchant/auth-logs');
        if (!res.ok) throw new Error('加载放行日志失败');
        
        state.auditLogs = await res.json();
        state.auditPage = 1;
        renderAuditLogs();
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="6" class="text-center text-danger">⚠️ 加载失败</td></tr>';
    }
}

// 渲染审计放行日志
function renderAuditLogs() {
    const tbody = document.getElementById('audit-logs-table-body');
    
    // 1. 过滤与搜索逻辑 (按酒店过滤、按手机/MAC/IP搜索)
    const hotelVal = state.auditHotelId;
    const searchVal = state.auditSearch.toLowerCase().trim();
    
    const filtered = state.auditLogs.filter(item => {
        if (hotelVal && String(item.hotelId) !== hotelVal) return false;
        if (searchVal) {
            const phone = String(item.phone).toLowerCase();
            const mac = String(item.mac).toLowerCase();
            const ip = String(item.ip).toLowerCase();
            return phone.includes(searchVal) || mac.includes(searchVal) || ip.includes(searchVal);
        }
        return true;
    });
    
    if (filtered.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" class="text-center">ℹ️ 暂无符合条件的连网审计流水</td></tr>';
        updatePaginationBar('audit-pagination', 0, 1, 20, (page) => {
            state.auditPage = page;
            renderAuditLogs();
        });
        return;
    }
    
    // 2. 分页逻辑 (默认一页20条)
    const pageSize = 20;
    const totalCount = filtered.length;
    const totalPages = Math.ceil(totalCount / pageSize);
    
    if (state.auditPage > totalPages) state.auditPage = totalPages;
    if (state.auditPage < 1) state.auditPage = 1;
    
    const startIndex = (state.auditPage - 1) * pageSize;
    const endIndex = Math.min(startIndex + pageSize, totalCount);
    const pageItems = filtered.slice(startIndex, endIndex);
    
    tbody.innerHTML = '';
    pageItems.forEach(item => {
        const tr = document.createElement('tr');
        const statusBadge = item.status === 'success' 
            ? '<span class="badge" style="background: rgba(16, 185, 129, 0.08); border-color: rgba(16, 185, 129, 0.25); color: var(--accent-glow);">放行成功</span>' 
            : `<span class="badge" style="background: rgba(239, 68, 68, 0.08); border-color: rgba(239, 68, 68, 0.25); color: var(--error-glow);">拦截 (${escapeHTML(item.status)})</span>`;
            
        tr.innerHTML = `
            <td>${item.hotelId}</td>
            <td><strong>${escapeHTML(item.phone)}</strong></td>
            <td><code>${escapeHTML(item.mac)}</code></td>
            <td><code>${escapeHTML(item.ip)}</code></td>
            <td>${statusBadge}</td>
            <td>${formatTime(item.created_at)}</td>
        `;
        tbody.appendChild(tr);
    });
    
    updatePaginationBar('audit-pagination', totalCount, state.auditPage, pageSize, (page) => {
        state.auditPage = page;
        renderAuditLogs();
    });
}

// 3.5 短信扣费账单
async function loadSMSLogs() {
    const tbody = document.getElementById('sms-logs-table-body');
    tbody.innerHTML = '<tr><td colspan="9" class="text-center">🔄 加载短信详单...</td></tr>';
    
    try {
        const res = await fetch('/api/merchant/sms-logs');
        if (!res.ok) throw new Error('加载短信详单失败');
        
        state.smsLogs = await res.json();
        state.smsPage = 1;
        renderSMSLogs();
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="9" class="text-center text-danger">⚠️ 加载失败</td></tr>';
    }
}

// 渲染短信扣费详单
function renderSMSLogs() {
    const tbody = document.getElementById('sms-logs-table-body');
    
    // 1. 过滤逻辑 (按酒店过滤)
    const hotelVal = state.smsHotelId;
    const filtered = state.smsLogs.filter(item => {
        if (hotelVal && String(item.hotelId) !== hotelVal) return false;
        return true;
    });
    
    if (filtered.length === 0) {
        tbody.innerHTML = '<tr><td colspan="9" class="text-center">ℹ️ 暂无符合条件的短信扣费记录</td></tr>';
        updatePaginationBar('sms-pagination', 0, 1, 20, (page) => {
            state.smsPage = page;
            renderSMSLogs();
        });
        return;
    }
    
    // 2. 分页逻辑 (一页20条)
    const pageSize = 20;
    const totalCount = filtered.length;
    const totalPages = Math.ceil(totalCount / pageSize);
    
    if (state.smsPage > totalPages) state.smsPage = totalPages;
    if (state.smsPage < 1) state.smsPage = 1;
    
    const startIndex = (state.smsPage - 1) * pageSize;
    const endIndex = Math.min(startIndex + pageSize, totalCount);
    const pageItems = filtered.slice(startIndex, endIndex);
    
    // 如果是超级管理员，动态显示“所属商户”列表头和单元格
    const isSuperAdmin = state.level >= 50;
    document.querySelectorAll('#pane-sms-logs .col-user').forEach(el => {
        el.style.display = isSuperAdmin ? 'table-cell' : 'none';
    });
    
    tbody.innerHTML = '';
    pageItems.forEach(item => {
        const tr = document.createElement('tr');
        
        const typeBadge = item.billing_type === 'package' 
            ? '<span class="badge" style="background: rgba(108, 92, 203, 0.08); border-color: rgba(108, 92, 203, 0.25); color: #6c5ce7;">扣减套餐</span>' 
            : '<span class="badge" style="background: rgba(9, 132, 227, 0.08); border-color: rgba(9, 132, 227, 0.25); color: #0984e3;">扣减余额</span>';
            
        tr.innerHTML = `
            <td>${item.hotelId}</td>
            <td class="col-user" style="display: ${isSuperAdmin ? 'table-cell' : 'none'};"><strong>${item.user || '-'}</strong></td>
            <td><strong>${escapeHTML(item.phone)}</strong></td>
            <td><code>${escapeHTML(item.ip)}</code></td>
            <td>${typeBadge}</td>
            <td>${item.deducted_count} 条</td>
            <td class="${item.deducted_amount > 0 ? 'text-accent' : ''}">${(item.deducted_amount / 100).toFixed(2)} 元</td>
            <td><span class="badge">${escapeHTML(item.provider)}</span></td>
            <td>${formatTime(item.created_at)}</td>
        `;
        tbody.appendChild(tr);
    });
    
    updatePaginationBar('sms-pagination', totalCount, state.smsPage, pageSize, (page) => {
        state.smsPage = page;
        renderSMSLogs();
    });
}

// 3.6 充值流水
async function loadRechargeLogs() {
    const tbody = document.getElementById('recharge-logs-table-body');
    tbody.innerHTML = '<tr><td colspan="7" class="text-center">🔄 获取对账流水...</td></tr>';
    
    try {
        const res = await fetch('/api/merchant/recharges');
        if (!res.ok) throw new Error('获取流水失败');
        
        state.rechargeLogs = await res.json();
        state.rechargePage = 1;
        renderRechargeLogs();
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="7" class="text-center text-danger">⚠️ 加载失败</td></tr>';
    }
}

// 渲染充值流水
function renderRechargeLogs() {
    const tbody = document.getElementById('recharge-logs-table-body');
    
    if (state.rechargeLogs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="text-center">ℹ️ 暂无历史充值流水</td></tr>';
        updatePaginationBar('recharge-pagination', 0, 1, 20, (page) => {
            state.rechargePage = page;
            renderRechargeLogs();
        });
        return;
    }
    
    // 分页逻辑 (一页20条)
    const pageSize = 20;
    const totalCount = state.rechargeLogs.length;
    const totalPages = Math.ceil(totalCount / pageSize);
    
    if (state.rechargePage > totalPages) state.rechargePage = totalPages;
    if (state.rechargePage < 1) state.rechargePage = 1;
    
    const startIndex = (state.rechargePage - 1) * pageSize;
    const endIndex = Math.min(startIndex + pageSize, totalCount);
    const pageItems = state.rechargeLogs.slice(startIndex, endIndex);
    
    tbody.innerHTML = '';
    pageItems.forEach(item => {
        const tr = document.createElement('tr');
        
        const typeText = item.type === 'package' 
            ? '<span class="badge" style="background: rgba(108, 92, 203, 0.08); border-color: rgba(108, 92, 203, 0.25); color: #6c5ce7;">购买套餐包</span>' 
            : '<span class="badge" style="background: rgba(16, 185, 129, 0.08); border-color: rgba(16, 185, 129, 0.25); color: var(--accent-glow);">余额直充</span>';
            
        tr.innerHTML = `
            <td><small class="text-muted">${item.orderId}</small></td>
            <td><strong>${item.user}</strong></td>
            <td>${typeText}</td>
            <td class="text-accent"><strong>${(item.amount / 100).toFixed(2)}</strong> 元</td>
            <td>${item.sms_count} 条</td>
            <td>${escapeHTML(item.package_name || '-')}</td>
            <td>${formatTime(item.created_at)}</td>
        `;
        tbody.appendChild(tr);
    });
    
    updatePaginationBar('recharge-pagination', totalCount, state.rechargePage, pageSize, (page) => {
        state.rechargePage = page;
        renderRechargeLogs();
    });
}

// 动态填充和维护酒店过滤器选项
function updateHotelFilters() {
    const auditFilter = document.getElementById('audit-hotel-filter');
    const smsFilter = document.getElementById('sms-hotel-filter');
    
    if (!auditFilter || !smsFilter) return;
    
    const currentAuditVal = auditFilter.value;
    const currentSmsVal = smsFilter.value;
    
    auditFilter.innerHTML = '<option value="">全部酒店</option>';
    smsFilter.innerHTML = '<option value="">全部酒店</option>';
    
    state.hotels.forEach(h => {
        const opt = document.createElement('option');
        opt.value = h.hotelId;
        opt.innerText = `${h.name} (ID: ${h.hotelId})`;
        
        auditFilter.appendChild(opt.cloneNode(true));
        smsFilter.appendChild(opt);
    });
    
    // 恢复先前选中的项
    if (state.hotels.some(h => String(h.hotelId) === currentAuditVal)) {
        auditFilter.value = currentAuditVal;
    } else {
        state.auditHotelId = '';
    }
    
    if (state.hotels.some(h => String(h.hotelId) === currentSmsVal)) {
        smsFilter.value = currentSmsVal;
    } else {
        state.smsHotelId = '';
    }
}

// 通用前端分页控制条生成逻辑
function updatePaginationBar(containerId, totalCount, currentPage, pageSize, onPageChange) {
    const bar = document.getElementById(containerId);
    if (!bar) return;
    
    const info = bar.querySelector('.pagination-info');
    const buttonsContainer = bar.querySelector('.pagination-buttons');
    
    const totalPages = Math.ceil(totalCount / pageSize) || 1;
    const fromIndex = totalCount === 0 ? 0 : (currentPage - 1) * pageSize + 1;
    const toIndex = Math.min(currentPage * pageSize, totalCount);
    
    info.innerText = `显示 ${fromIndex} 到 ${toIndex} 条，共 ${totalCount} 条`;
    buttonsContainer.innerHTML = '';
    
    // 上一页
    const btnPrev = document.createElement('button');
    btnPrev.className = 'pagination-btn';
    btnPrev.innerText = '◀';
    btnPrev.disabled = currentPage === 1;
    btnPrev.addEventListener('click', () => onPageChange(currentPage - 1));
    buttonsContainer.appendChild(btnPrev);
    
    // 页码页码
    let startPage = 1;
    let endPage = totalPages;
    if (totalPages > 5) {
        if (currentPage <= 3) {
            endPage = 5;
        } else if (currentPage >= totalPages - 2) {
            startPage = totalPages - 4;
        } else {
            startPage = currentPage - 2;
            endPage = currentPage + 2;
        }
    }
    
    for (let p = startPage; p <= endPage; p++) {
        const btnPage = document.createElement('button');
        btnPage.className = `pagination-btn ${p === currentPage ? 'active' : ''}`;
        btnPage.innerText = p;
        btnPage.addEventListener('click', () => onPageChange(p));
        buttonsContainer.appendChild(btnPage);
    }
    
    // 下一页
    const btnNext = document.createElement('button');
    btnNext.className = 'pagination-btn';
    btnNext.innerText = '▶';
    btnNext.disabled = currentPage === totalPages;
    btnNext.addEventListener('click', () => onPageChange(currentPage + 1));
    buttonsContainer.appendChild(btnNext);
}

// 3.7 超管专区: 商户管理
async function loadSuperUsers() {
    const tbody = document.getElementById('super-users-table-body');
    tbody.innerHTML = '<tr><td colspan="6" class="text-center">🔄 读取全网商户数据...</td></tr>';
    
    try {
        const res = await fetch('/api/admin/users');
        if (!res.ok) throw new Error('拉取商户失败');
        
        const list = await res.json();
        tbody.innerHTML = '';
        list.forEach(u => {
            const tr = document.createElement('tr');
            const levelBadge = u.level >= 50 
                ? '<span class="badge badge-admin">超级管理员</span>' 
                : '<span class="badge">酒店商户</span>';
                
            tr.innerHTML = `
                <td><strong>${u.user}</strong></td>
                <td>${levelBadge}</td>
                <td class="text-accent"><strong>${(u.balance / 100).toFixed(2)}</strong> 元</td>
                <td><strong>${u.sms_count}</strong> 条</td>
                <td>${formatTime(u.created_at)}</td>
                <td>
                    <button class="btn-sm" style="border-color: var(--primary-accent); color: var(--primary-accent);" onclick="openRechargeModal(${u.user})">💳 注资充值</button>
                    <button class="btn-sm" onclick="openEditUserModal(${u.user}, ${u.level}, ${u.balance}, ${u.sms_count})">⚙️ 修改</button>
                    <button class="btn-sm btn-danger" onclick="deleteUser(${u.user})">🗑️ 删除</button>
                </td>
            `;
            tbody.appendChild(tr);
        });
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="6" class="text-center text-danger">⚠️ 加载失败</td></tr>';
    }
}

// 3.8 超管专区: 套餐维护
async function loadSuperPackages() {
    const tbody = document.getElementById('super-packages-table-body');
    tbody.innerHTML = '<tr><td colspan="6" class="text-center">🔄 获取运营套餐规则...</td></tr>';
    
    try {
        const res = await fetch('/api/admin/packages');
        if (!res.ok) throw new Error('拉取套餐包失败');
        
        state.packages = await res.json();
        tbody.innerHTML = '';
        
        if (state.packages.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="text-center">ℹ️ 暂无套餐配置。</td></tr>';
            return;
        }
        
        state.packages.forEach(p => {
            const tr = document.createElement('tr');
            
            tr.innerHTML = `
                <td><code>${escapeHTML(p.packageId)}</code></td>
                <td><strong>${escapeHTML(p.name)}</strong></td>
                <td>${p.price} 分 (${(p.price / 100).toFixed(2)} 元)</td>
                <td>${p.sms_count} 条</td>
                <td>${p.status === 1 ? '<span class="badge" style="background: rgba(16, 185, 129, 0.08); border-color: rgba(16, 185, 129, 0.25); color: var(--accent-glow);">● 上架中</span>' : '<span class="badge" style="background: rgba(239, 68, 68, 0.08); border-color: rgba(239, 68, 68, 0.25); color: var(--error-glow);">● 已下架</span>'}</td>
                <td>
                    <button class="btn-sm" onclick="openEditPackageModal('${p.packageId}')">⚙️ 维护</button>
                    <button class="btn-sm btn-danger" onclick="deletePackage('${p.packageId}')">🗑️ 物理删除</button>
                </td>
            `;
            tbody.appendChild(tr);
        });
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="6" class="text-center text-danger">⚠️ 加载失败</td></tr>';
    }
}

// 3.9 超管专区: 短信通道池
async function loadSuperSMSProviders() {
    const tbody = document.getElementById('super-sms-providers-table-body');
    tbody.innerHTML = '<tr><td colspan="6" class="text-center">🔄 读取系统短信通道状态...</td></tr>';
    
    try {
        const res = await fetch('/api/admin/sms-providers');
        if (!res.ok) throw new Error('拉取短信通道失败');
        
        state.smsProviders = await res.json();
        tbody.innerHTML = '';
        
        if (state.smsProviders.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="text-center">ℹ️ 暂无短信通道配置，请新建通道。</td></tr>';
            return;
        }
        
        state.smsProviders.forEach(p => {
            const tr = document.createElement('tr');
            
            // 构造动态配置列表显示 (隐藏敏感 key)
            let configHtml = '';
            if (p.config) {
                for (let k in p.config) {
                    configHtml += `<div style="font-size: 11px;"><strong style="color: var(--primary-accent);">${k}:</strong> <code>${escapeHTML(p.config[k])}</code></div>`;
                }
            }
            
            // 通道状态 iOS 滑动 Toggle 按钮设计
            const isChecked = p.status === 1 ? 'checked' : '';
            const statusToggleHtml = `
                <label class="switch">
                    <input type="checkbox" ${isChecked} onchange="toggleSMSProviderStatus('${p.id}', this.checked)">
                    <span class="slider"></span>
                </label>
            `;
            
            // 通道类型 Tag 样式
            const typeBadge = {
                'mock': '<span class="badge" style="background: rgba(100, 116, 139, 0.08); border-color: rgba(100, 116, 139, 0.25); color: var(--text-muted);">模拟通道 (Mock)</span>',
                'aliyun': '<span class="badge" style="background: rgba(255, 118, 117, 0.08); border-color: rgba(255, 118, 117, 0.25); color: #ff7675;">阿里云短信 (Aliyun)</span>',
                'tencent': '<span class="badge" style="background: rgba(9, 132, 227, 0.08); border-color: rgba(9, 132, 227, 0.25); color: #0984e3;">腾讯云短信 (Tencent)</span>',
                'ihuyi': '<span class="badge" style="background: rgba(16, 185, 129, 0.08); border-color: rgba(16, 185, 129, 0.25); color: #10b981;">互亿无线 (Ihuyi)</span>'
            }[p.provider] || `<span class="badge">${p.provider}</span>`;

            tr.innerHTML = `
                <td><strong>${typeBadge}</strong></td>
                <td><strong>${p.weight}</strong></td>
                <td>${statusToggleHtml}</td>
                <td>${configHtml || '-'}</td>
                <td>${formatTime(p.created_at)}</td>
                <td>
                    <button class="btn-sm" onclick="openEditSMSProviderModal('${p.id}')">⚙️ 编辑</button>
                    <button class="btn-sm btn-danger" onclick="deleteSMSProvider('${p.id}')">🗑️ 物理删除</button>
                </td>
            `;
            tbody.appendChild(tr);
        });
    } catch (err) {
        tbody.innerHTML = '<tr><td colspan="6" class="text-center text-danger">⚠️ 加载失败</td></tr>';
    }
}

// 快速启用/停用通道接口
window.toggleSMSProviderStatus = async function(id, checked) {
    const provider = state.smsProviders.find(p => p.id === id);
    if (!provider) return;
    
    const payload = {
        id: id,
        provider: provider.provider,
        weight: provider.weight,
        status: checked ? 1 : 0,
        config: provider.config || {}
    };

    try {
        const res = await fetch('/api/admin/sms-providers', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        
        if (res.ok) {
            showToast(`✅ 通道状态修改成功！已${checked ? '启用' : '禁用'}`, 'success');
            // 重新刷新局部列表
            await loadSuperSMSProviders();
        } else {
            showToast('❌ 通道状态更新失败', 'danger');
        }
    } catch (e) {
        showToast('❌ 网络连接错误', 'danger');
    }
};

// =========================================================================
// 4. 事件监听绑定与表单拦截处理 (Modals and Actions)
// =========================================================================

function setupEventListeners() {
    // 登录事件
    document.getElementById('btn-login').addEventListener('click', executeLogin);
    document.getElementById('login-password').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') executeLogin();
    });

    // 注销事件
    document.getElementById('btn-logout').addEventListener('click', executeLogout);

    // 酒店模态框控制
    document.getElementById('btn-add-hotel').addEventListener('click', () => {
        openAddHotelModal();
    });
    document.getElementById('btn-close-hotel-modal').addEventListener('click', () => {
        document.getElementById('modal-hotel').style.display = 'none';
    });
    document.getElementById('btn-save-hotel').addEventListener('click', saveHotel);

    // 商户账号模态框控制
    document.getElementById('btn-add-user').addEventListener('click', () => {
        document.getElementById('modal-user').style.display = 'flex';
        // 清空表单
        document.getElementById('user-account-input').value = '';
        document.getElementById('user-account-input').disabled = false;
        document.getElementById('user-password-input').value = '';
        document.getElementById('user-level-select').value = '10';
    });
    document.getElementById('btn-close-user-modal').addEventListener('click', () => {
        document.getElementById('modal-user').style.display = 'none';
    });
    document.getElementById('btn-save-user').addEventListener('click', saveUser);

    // 充值模态框控制
    document.getElementById('recharge-type-select').addEventListener('change', (e) => {
        const type = e.target.value;
        if (type === 'package') {
            document.getElementById('recharge-sms-group').style.display = 'block';
            document.getElementById('recharge-pkgname-group').style.display = 'block';
        } else {
            document.getElementById('recharge-sms-group').style.display = 'none';
            document.getElementById('recharge-pkgname-group').style.display = 'none';
        }
    });
    document.getElementById('btn-close-recharge-modal').addEventListener('click', () => {
        document.getElementById('modal-recharge').style.display = 'none';
    });
    document.getElementById('btn-save-recharge').addEventListener('click', saveManualRecharge);

    // 套餐维护模态框控制
    document.getElementById('btn-add-package').addEventListener('click', () => {
        document.getElementById('modal-package').style.display = 'flex';
        document.getElementById('pkg-id-input').value = '';
        document.getElementById('pkg-id-input').disabled = false;
        document.getElementById('pkg-name-input').value = '';
        document.getElementById('pkg-price-input').value = '';
        document.getElementById('pkg-sms-input').value = '';
        state.isEditingPackage = false;
    });
    document.getElementById('btn-close-package-modal').addEventListener('click', () => {
        document.getElementById('modal-package').style.display = 'none';
    });
    document.getElementById('btn-save-package').addEventListener('click', savePackage);

    // 短信通道模态框联动切换
    document.getElementById('sms-provider-type-select').addEventListener('change', (e) => {
        toggleSMSProviderConfigFields(e.target.value);
    });
    
    // 短信通道模态框显示
    document.getElementById('btn-add-sms-provider').addEventListener('click', () => {
        openAddSMSProviderModal();
    });
    document.getElementById('btn-close-sms-provider-modal').addEventListener('click', () => {
        document.getElementById('modal-sms-provider').style.display = 'none';
    });
    document.getElementById('btn-save-sms-provider').addEventListener('click', saveSMSProvider);

    // =========================================================================
    // 列表搜索、过滤与输入联动事件绑定
    // =========================================================================
    
    // 1. 酒店关键字搜索
    const hotelSearch = document.getElementById('hotel-search-input');
    if (hotelSearch) {
        hotelSearch.addEventListener('input', (e) => {
            state.hotelsSearch = e.target.value;
            state.hotelsPage = 1;
            renderHotels();
        });
    }
    
    // 2. 审计日志酒店过滤和搜索
    const auditHotelFilter = document.getElementById('audit-hotel-filter');
    if (auditHotelFilter) {
        auditHotelFilter.addEventListener('change', (e) => {
            state.auditHotelId = e.target.value;
            state.auditPage = 1;
            renderAuditLogs();
        });
    }
    
    const auditSearch = document.getElementById('audit-search-input');
    if (auditSearch) {
        auditSearch.addEventListener('input', (e) => {
            state.auditSearch = e.target.value;
            state.auditPage = 1;
            renderAuditLogs();
        });
    }
    
    // 3. 短信明细酒店过滤
    const smsHotelFilter = document.getElementById('sms-hotel-filter');
    if (smsHotelFilter) {
        smsHotelFilter.addEventListener('change', (e) => {
            state.smsHotelId = e.target.value;
            state.smsPage = 1;
            renderSMSLogs();
        });
    }

    // =========================================================================
    // 4. 对照自助注册模块事件绑定
    // =========================================================================
    
    // 跳转至注册界面
    const linkGotoReg = document.getElementById('link-goto-register');
    if (linkGotoReg) {
        linkGotoReg.addEventListener('click', () => {
            document.getElementById('login-container').style.display = 'none';
            document.getElementById('register-container').style.display = 'flex';
            document.getElementById('reg-error').style.display = 'none';
            // 清空表单
            document.getElementById('reg-phone').value = '';
            document.getElementById('reg-code').value = '';
            document.getElementById('reg-password').value = '';
            document.getElementById('reg-confirm-password').value = '';
        });
    }

    // 返回登录界面
    const linkGotoLogin = document.getElementById('link-goto-login');
    if (linkGotoLogin) {
        linkGotoLogin.addEventListener('click', () => {
            document.getElementById('login-container').style.display = 'flex';
            document.getElementById('register-container').style.display = 'none';
            document.getElementById('login-error').style.display = 'none';
        });
    }

    // 获取验证码
    const btnRegSendCode = document.getElementById('btn-reg-send-code');
    if (btnRegSendCode) {
        btnRegSendCode.addEventListener('click', executeRegisterSendSMS);
    }

    // 提交自助注册
    const btnRegisterSubmit = document.getElementById('btn-register-submit');
    if (btnRegisterSubmit) {
        btnRegisterSubmit.addEventListener('click', executeRegisterSubmit);
    }
}

// 4.1 登录操作
async function executeLogin() {
    const userVal = document.getElementById('login-user').value.trim();
    const pwdVal = document.getElementById('login-password').value;
    const errDiv = document.getElementById('login-error');
    
    if (!userVal || !pwdVal) {
        errDiv.innerText = '⚠️ 请填写账号和密码';
        errDiv.style.display = 'block';
        return;
    }
    
    const userInt = parseInt(userVal, 10);
    if (isNaN(userInt)) {
        errDiv.innerText = '⚠️ 账号必须是纯数字（手机号或超管账号）';
        errDiv.style.display = 'block';
        return;
    }
    
    errDiv.style.display = 'none';
    
    try {
        const res = await fetch('/api/admin/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ user: userInt, password: pwdVal })
        });
        
        const data = await res.json();
        if (res.ok) {
            showToast('⚡ 欢迎登录系统控制台！', 'success');
            await checkLoginStatus();
        } else {
            errDiv.innerText = `⚠️ ${data.error || '登录失败'}`;
            errDiv.style.display = 'block';
        }
    } catch (err) {
        errDiv.innerText = '⚠️ 网络连接失败';
        errDiv.style.display = 'block';
    }
}

// 4.2 注销操作
async function executeLogout() {
    if (!confirm('确定要退出 wifi认证系统吗？')) return;
    try {
        await fetch('/api/admin/logout', { method: 'POST' });
        showToast('🔓 退出成功', 'secondary');
        showLoginOverlay();
    } catch (err) {
        console.error('注销异常:', err);
    }
}

// 4.3 酒店新建与编辑
function openAddHotelModal() {
    state.isEditingHotel = false;
    document.getElementById('modal-hotel-title').innerText = '新建对接酒店 (系统自动分配ID)';
    document.getElementById('hotel-form-id').value = '';
    
    document.getElementById('hotel-name-input').value = '';
    document.getElementById('hotel-gateway-select').value = 'ikuai';
    document.getElementById('hotel-appkey-input').value = '';
    document.getElementById('hotel-custom-input').value = '';
    document.getElementById('hotel-welcome-input').value = '';
    document.getElementById('hotel-sms-cooldown-input').value = '60'; // 默认60秒冷却
    document.getElementById('hotel-ip-cooldown-input').value = '60';  // 默认60秒冷却
    document.getElementById('hotel-max-sends-day-input').value = '0';  // 默认0代表不限制
    document.getElementById('hotel-bypass-auth-select').value = '0';   // 默认禁用二次免密认证
    
    const userField = document.getElementById('hotel-user-input');
    // 如果是普通商户，强行预设并锁定归属商户为自身账号
    if (state.level < 50) {
        userField.value = state.user || '';
        userField.disabled = true;
    } else {
        userField.value = '';
        userField.disabled = false;
    }
    
    document.getElementById('modal-hotel').style.display = 'flex';
}

window.openEditHotelModal = function(hotelId) {
    state.isEditingHotel = true;
    const hotel = state.hotels.find(h => h.hotelId === hotelId);
    if (!hotel) return;
    
    document.getElementById('modal-hotel-title').innerText = `配置维护 - 酒店 ID: ${hotelId}`;
    document.getElementById('hotel-form-id').value = hotelId;
    
    document.getElementById('hotel-name-input').value = hotel.name;
    document.getElementById('hotel-gateway-select').value = hotel.gateway_type;
    document.getElementById('hotel-appkey-input').value = hotel.app_key || '';
    document.getElementById('hotel-custom-input').value = hotel.custom_name || '';
    document.getElementById('hotel-welcome-input').value = hotel.welcome_text || '';
    document.getElementById('hotel-sms-cooldown-input').value = hotel.sms_cooldown || '60';
    document.getElementById('hotel-ip-cooldown-input').value = hotel.ip_cooldown || '60';
    document.getElementById('hotel-max-sends-day-input').value = hotel.max_sends_day || '0';
    document.getElementById('hotel-bypass-auth-select').value = hotel.bypass_auth || '0';
    
    const userField = document.getElementById('hotel-user-input');
    userField.value = hotel.user;
    
    // 如果不是超级管理员，不允许修改酒店归属用户
    if (state.level < 50) {
        userField.disabled = true;
    } else {
        userField.disabled = false;
    }
    
    document.getElementById('modal-hotel').style.display = 'flex';
};

async function saveHotel() {
    const hotelIdVal = document.getElementById('hotel-form-id').value;
    const name = document.getElementById('hotel-name-input').value.trim();
    const gateway = document.getElementById('hotel-gateway-select').value;
    const appkey = document.getElementById('hotel-appkey-input').value.trim();
    const custom = document.getElementById('hotel-custom-input').value.trim();
    const welcome = document.getElementById('hotel-welcome-input').value.trim();
    const smsCooldown = parseInt(document.getElementById('hotel-sms-cooldown-input').value, 10);
    const ipCooldown = parseInt(document.getElementById('hotel-ip-cooldown-input').value, 10);
    const maxSendsDay = parseInt(document.getElementById('hotel-max-sends-day-input').value, 10);
    const bypassAuth = parseInt(document.getElementById('hotel-bypass-auth-select').value, 10);
    const userStr = document.getElementById('hotel-user-input').value.trim();
    
    if (!name || !userStr) {
        showToast('⚠️ 酒店名称与归属商户为必填项！', 'danger');
        return;
    }
    
    const userInt = parseInt(userStr, 10);
    if (isNaN(userInt)) {
        showToast('⚠️ 归属商户必须是纯数字账号！', 'danger');
        return;
    }
    
    const payload = {
        name: name,
        gateway_type: gateway,
        app_key: appkey,
        custom_name: custom,
        welcome_text: welcome,
        sms_cooldown: isNaN(smsCooldown) ? 60 : smsCooldown,
        ip_cooldown: isNaN(ipCooldown) ? 60 : ipCooldown,
        max_sends_day: isNaN(maxSendsDay) ? 0 : maxSendsDay,
        bypass_auth: isNaN(bypassAuth) ? 0 : bypassAuth,
        user: userInt
    };
    
    try {
        let res;
        if (state.isEditingHotel) {
            payload.hotelId = parseInt(hotelIdVal, 10);
            payload.status = 1;
            // 商户和超管共同的修改接口为 PUT /api/merchant/hotels
            res = await fetch('/api/merchant/hotels', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        } else {
            // 新增酒店节点接口统一合并至商户专属接口路由，保证普通商户与超级管理员均可添加并自动完成权限校验
            res = await fetch('/api/merchant/hotels', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        }
        
        if (res.ok) {
            showToast('✅ 酒店网关配置保存成功！', 'success');
            document.getElementById('modal-hotel').style.display = 'none';
            await loadHotels();
        } else {
            const data = await res.json();
            showToast(`❌ 保存失败: ${data.error || '未知错误'}`, 'danger');
        }
    } catch (err) {
        showToast('❌ 网络错误，保存失败', 'danger');
    }
}

// 4.4 商户增设与修改
async function saveUser() {
    const accountStr = document.getElementById('user-account-input').value.trim();
    const pwd = document.getElementById('user-password-input').value;
    const levelInt = parseInt(document.getElementById('user-level-select').value, 10);
    
    if (!accountStr) {
        showToast('⚠️ 商户手机号/账号不能为空', 'danger');
        return;
    }
    
    const accountInt = parseInt(accountStr, 10);
    if (isNaN(accountInt)) {
        showToast('⚠️ 账号必须是纯数字', 'danger');
        return;
    }
    
    const isEditing = document.getElementById('user-account-input').disabled;
    
    try {
        let res;
        if (isEditing) {
            // 修改已有用户 (如修改余额、Level、重置密码等)
            const payload = {
                user: accountInt,
                level: levelInt
            };
            if (pwd) {
                payload.password = pwd;
            }
            res = await fetch('/api/admin/users', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        } else {
            // 新建用户
            if (!pwd) {
                showToast('⚠️ 新建商户必须设置初始密码', 'danger');
                return;
            }
            res = await fetch('/api/admin/users', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ user: accountInt, password: pwd, level: levelInt })
            });
        }
        
        if (res.ok) {
            showToast('✅ 商户管理保存成功！', 'success');
            document.getElementById('modal-user').style.display = 'none';
            await loadSuperUsers();
        } else {
            const data = await res.json();
            showToast(`❌ 保存失败: ${data.error || '未知错误'}`, 'danger');
        }
    } catch (err) {
        showToast('❌ 网络连接错误', 'danger');
    }
}

window.openEditUserModal = function(user, level, balance, smsCount) {
    document.getElementById('modal-user').style.display = 'flex';
    document.getElementById('user-account-input').value = user;
    document.getElementById('user-account-input').disabled = true;
    document.getElementById('user-password-input').value = '';
    document.getElementById('user-password-input').placeholder = '留空表示不修改密码';
    document.getElementById('user-level-select').value = level;
};

// 4.5 商户包/余额手动注资
window.openRechargeModal = function(user) {
    document.getElementById('recharge-user-hidden').value = user;
    document.getElementById('recharge-type-select').value = 'balance';
    document.getElementById('recharge-amount-input').value = '';
    document.getElementById('recharge-sms-input').value = '0';
    document.getElementById('recharge-pkgname-input').value = '系统手动充值';
    
    document.getElementById('recharge-sms-group').style.display = 'none';
    document.getElementById('recharge-pkgname-group').style.display = 'none';
    
    document.getElementById('modal-recharge').style.display = 'flex';
};

async function saveManualRecharge() {
    const user = parseInt(document.getElementById('recharge-user-hidden').value, 10);
    const type = document.getElementById('recharge-type-select').value;
    const amountVal = document.getElementById('recharge-amount-input').value.trim();
    const smsCount = parseInt(document.getElementById('recharge-sms-input').value, 10);
    const pkgName = document.getElementById('recharge-pkgname-input').value.trim();
    
    const amountCents = parseInt(amountVal, 10);
    
    if (isNaN(amountCents) || amountCents <= 0) {
        showToast('⚠️ 充值金额必须大于 0', 'danger');
        return;
    }
    
    const payload = {
        user: user,
        type: type,
        amount: amountCents,
        sms_count: type === 'package' ? smsCount : 0,
        package_name: type === 'package' ? pkgName : '系统注资账户余额'
    };
    
    try {
        const res = await fetch('/api/admin/recharge/manual', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        
        if (res.ok) {
            showToast('✅ 手动划拨充值成功，已秒级到账！', 'success');
            document.getElementById('modal-recharge').style.display = 'none';
            await loadSuperUsers();
        } else {
            const data = await res.json();
            showToast(`❌ 划拨失败: ${data.error || '未知错误'}`, 'danger');
        }
    } catch (err) {
        showToast('❌ 网络错误，充值失败', 'danger');
    }
}

// 4.6 运营套餐规则维护
window.openEditPackageModal = function(packageId) {
    state.isEditingPackage = true;
    const p = state.packages.find(pkg => pkg.packageId === packageId);
    if (!p) return;
    
    document.getElementById('pkg-id-input').value = p.packageId;
    document.getElementById('pkg-id-input').disabled = true;
    document.getElementById('pkg-name-input').value = p.name;
    document.getElementById('pkg-price-input').value = p.price;
    document.getElementById('pkg-sms-input').value = p.sms_count;
    
    document.getElementById('modal-package').style.display = 'flex';
};

async function savePackage() {
    const pkgId = document.getElementById('pkg-id-input').value.trim();
    const name = document.getElementById('pkg-name-input').value.trim();
    const priceCents = parseInt(document.getElementById('pkg-price-input').value, 10);
    const smsCount = parseInt(document.getElementById('pkg-sms-input').value, 10);
    
    if (!pkgId || !name || isNaN(priceCents) || isNaN(smsCount)) {
        showToast('⚠️ 套餐表单项必须完整填写', 'danger');
        return;
    }
    
    const payload = {
        packageId: pkgId,
        name: name,
        price: priceCents,
        sms_count: smsCount,
        status: 1
    };
    
    try {
        let res;
        if (state.isEditingPackage) {
            res = await fetch('/api/admin/packages', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        } else {
            res = await fetch('/api/admin/packages', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        }
        
        if (res.ok) {
            showToast('✅ 运营套餐规则维护成功！', 'success');
            document.getElementById('modal-package').style.display = 'none';
            await loadSuperPackages();
        } else {
            const data = await res.json();
            showToast(`❌ 套餐维护失败: ${data.error}`, 'danger');
        }
    } catch (err) {
        showToast('❌ 套餐维护网络故障', 'danger');
    }
}

window.deletePackage = async function(packageId) {
    if (!confirm(`⚠️ 危险警示：确定要物理删除该套餐规则(${packageId})吗？这不会影响已购买用户的短信额度。`)) return;
    
    try {
        const res = await fetch(`/api/admin/packages?packageId=${encodeURIComponent(packageId)}`, {
            method: 'DELETE'
        });
        
        if (res.ok) {
            showToast('🗑️ 套餐规则物理删除成功！', 'success');
            await loadSuperPackages();
        } else {
            const data = await res.json();
            showToast(`❌ 删除失败: ${data.error}`, 'danger');
        }
    } catch (err) {
        showToast('❌ 网络错误，删除失败', 'danger');
    }
};

// 4.7 商户自主购买套餐包
window.buyPackage = async function(packageId) {
    if (!confirm('确定要在商城内扣除您的账户余额，订购该短信套餐包吗？扣费后秒级到账生效。')) return;
    
    try {
        const res = await fetch('/api/merchant/buy-package', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ packageId: packageId })
        });
        
        const data = await res.json();
        if (res.ok) {
            showToast(`🎉 恭喜！成功购买套餐，累加 ${data.sms_count} 条短信！`, 'success');
            // 重新刷新 dashboard 钱包和数据
            await loadDashboardStats();
        } else {
            alert(`⚠️ 购买失败: ${data.error || '请核实账户余额。'}`);
        }
    } catch (err) {
        showToast('❌ 购买网络故障', 'danger');
    }
};

// =========================================================================
// 5. 短信通道模态控制与交互 CRUD
// =========================================================================

// 切换短信通道配置输入表单项
function toggleSMSProviderConfigFields(type) {
    // 隐藏所有配置块
    document.querySelectorAll('.sms-config-fields').forEach(el => el.style.display = 'none');
    
    // 显示对应的配置块
    const targetBlock = document.getElementById(`sms-config-${type}-fields`);
    if (targetBlock) {
        targetBlock.style.display = 'flex';
    }
}

// 打开新建短信通道弹框
function openAddSMSProviderModal() {
    state.isEditingSMSProvider = false;
    document.getElementById('modal-sms-provider-title').innerText = '新建对接短信通道';
    document.getElementById('sms-provider-form-id').value = '';
    
    // 默认值
    document.getElementById('sms-provider-type-select').value = 'mock';
    document.getElementById('sms-provider-type-select').disabled = false;
    document.getElementById('sms-provider-weight-input').value = '5';
    document.getElementById('sms-provider-status-select').value = '1';
    
    // 清空配置项
    document.getElementById('sms-config-mock-signname').value = '系统统一认证';
    
    document.getElementById('sms-config-aliyun-keyid').value = '';
    document.getElementById('sms-config-aliyun-secret').value = '';
    document.getElementById('sms-config-aliyun-secret').placeholder = '输入密钥 (AccessKeySecret)';
    document.getElementById('sms-config-aliyun-signname').value = '';
    document.getElementById('sms-config-aliyun-tplcode').value = '';
    
    document.getElementById('sms-config-tencent-secretid').value = '';
    document.getElementById('sms-config-tencent-secretkey').value = '';
    document.getElementById('sms-config-tencent-secretkey').placeholder = '输入密钥 (SecretKey)';
    document.getElementById('sms-config-tencent-appid').value = '';
    document.getElementById('sms-config-tencent-signname').value = '';
    document.getElementById('sms-config-tencent-tplid').value = '';

    document.getElementById('sms-config-ihuyi-apiid').value = '';
    document.getElementById('sms-config-ihuyi-apikey').value = '';
    document.getElementById('sms-config-ihuyi-apikey').placeholder = '输入密钥 (APIKEY)';
    document.getElementById('sms-config-ihuyi-tplid').value = '';
    
    toggleSMSProviderConfigFields('mock');
    document.getElementById('modal-sms-provider').style.display = 'flex';
}

// 打开编辑短信通道弹窗
window.openEditSMSProviderModal = function(id) {
    state.isEditingSMSProvider = true;
    const provider = state.smsProviders.find(p => p.id === id);
    if (!provider) return;
    
    document.getElementById('modal-sms-provider-title').innerText = `编辑维护短信通道`;
    document.getElementById('sms-provider-form-id').value = id;
    
    document.getElementById('sms-provider-type-select').value = provider.provider;
    document.getElementById('sms-provider-type-select').disabled = true; // 不允许修改类型类型
    document.getElementById('sms-provider-weight-input').value = provider.weight;
    document.getElementById('sms-provider-status-select').value = provider.status;
    
    // 联动配置表单并回填
    toggleSMSProviderConfigFields(provider.provider);
    
    const cfg = provider.config || {};
    if (provider.provider === 'mock') {
        document.getElementById('sms-config-mock-signname').value = cfg.sign_name || '';
    } else if (provider.provider === 'aliyun') {
        document.getElementById('sms-config-aliyun-keyid').value = cfg.access_key_id || '';
        document.getElementById('sms-config-aliyun-secret').value = ''; // 默认为空，掩码防泄露
        document.getElementById('sms-config-aliyun-secret').placeholder = '****** (留空表示不修改已有密钥)';
        document.getElementById('sms-config-aliyun-signname').value = cfg.sign_name || '';
        document.getElementById('sms-config-aliyun-tplcode').value = cfg.template_code || '';
    } else if (provider.provider === 'tencent') {
        document.getElementById('sms-config-tencent-secretid').value = cfg.secret_id || '';
        document.getElementById('sms-config-tencent-secretkey').value = '';
        document.getElementById('sms-config-tencent-secretkey').placeholder = '****** (留空表示不修改已有密钥)';
        document.getElementById('sms-config-tencent-appid').value = cfg.sdk_app_id || '';
        document.getElementById('sms-config-tencent-signname').value = cfg.sign_name || '';
        document.getElementById('sms-config-tencent-tplid').value = cfg.template_id || '';
    } else if (provider.provider === 'ihuyi') {
        document.getElementById('sms-config-ihuyi-apiid').value = cfg.api_id || '';
        document.getElementById('sms-config-ihuyi-apikey').value = '';
        document.getElementById('sms-config-ihuyi-apikey').placeholder = '****** (留空表示不修改已有密钥)';
        document.getElementById('sms-config-ihuyi-tplid').value = cfg.template_id || '';
    }
    
    document.getElementById('modal-sms-provider').style.display = 'flex';
};

// 保存通道数据
async function saveSMSProvider() {
    const idVal = document.getElementById('sms-provider-form-id').value;
    const providerType = document.getElementById('sms-provider-type-select').value;
    const weightVal = parseInt(document.getElementById('sms-provider-weight-input').value, 10);
    const statusVal = parseInt(document.getElementById('sms-provider-status-select').value, 10);
    
    if (isNaN(weightVal) || weightVal < 1 || weightVal > 10) {
        showToast('⚠️ 发送权重必须为 1-10 之间的整数', 'danger');
        return;
    }
    
    // 构造各个通道配置的 payload
    const config = {};
    if (providerType === 'mock') {
        const sign = document.getElementById('sms-config-mock-signname').value.trim();
        if (!sign) {
            showToast('⚠️ 短信签名不能为空', 'danger');
            return;
        }
        config.sign_name = sign;
    } else if (providerType === 'aliyun') {
        const keyid = document.getElementById('sms-config-aliyun-keyid').value.trim();
        let secret = document.getElementById('sms-config-aliyun-secret').value;
        const sign = document.getElementById('sms-config-aliyun-signname').value.trim();
        const tpl = document.getElementById('sms-config-aliyun-tplcode').value.trim();
        
        if (!keyid || !sign || !tpl) {
            showToast('⚠️ 请完整填写阿里云通道的主要参数', 'danger');
            return;
        }
        
        if (!state.isEditingSMSProvider && !secret) {
            showToast('⚠️ 新建通道必须输入密钥', 'danger');
            return;
        }
        
        if (state.isEditingSMSProvider && !secret) {
            // 如果留空，直接上传 "******" 告诉后端保留原值
            secret = "******";
        }
        
        config.access_key_id = keyid;
        config.access_key_secret = secret;
        config.sign_name = sign;
        config.template_code = tpl;
    } else if (providerType === 'tencent') {
        const secretid = document.getElementById('sms-config-tencent-secretid').value.trim();
        let secretkey = document.getElementById('sms-config-tencent-secretkey').value;
        const appid = document.getElementById('sms-config-tencent-appid').value.trim();
        const sign = document.getElementById('sms-config-tencent-signname').value.trim();
        const tpl = document.getElementById('sms-config-tencent-tplid').value.trim();
        
        if (!secretid || !appid || !sign || !tpl) {
            showToast('⚠️ 请完整填写腾讯云通道的主要参数', 'danger');
            return;
        }
        
        if (!state.isEditingSMSProvider && !secretkey) {
            showToast('⚠️ 新建通道必须输入密钥', 'danger');
            return;
        }
        
        if (state.isEditingSMSProvider && !secretkey) {
            secretkey = "******";
        }
        
        config.secret_id = secretid;
        config.secret_key = secretkey;
        config.sdk_app_id = appid;
        config.sign_name = sign;
        config.template_id = tpl;
    } else if (providerType === 'ihuyi') {
        const apiid = document.getElementById('sms-config-ihuyi-apiid').value.trim();
        let apikey = document.getElementById('sms-config-ihuyi-apikey').value;
        const tpl = document.getElementById('sms-config-ihuyi-tplid').value.trim();
        
        if (!apiid) {
            showToast('⚠️ 请填写互亿无线 API ID', 'danger');
            return;
        }
        
        if (!state.isEditingSMSProvider && !apikey) {
            showToast('⚠️ 新建通道必须输入密钥', 'danger');
            return;
        }
        
        if (state.isEditingSMSProvider && !apikey) {
            apikey = "******";
        }
        
        config.api_id = apiid;
        config.api_key = apikey;
        config.template_id = tpl;
    }
    
    const payload = {
        provider: providerType,
        weight: weightVal,
        status: statusVal,
        config: config
    };
    
    try {
        let res;
        if (state.isEditingSMSProvider) {
            payload.id = idVal;
            res = await fetch('/api/admin/sms-providers', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        } else {
            res = await fetch('/api/admin/sms-providers', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
        }
        
        if (res.ok) {
            showToast('✅ 短信通道配置保存成功！', 'success');
            document.getElementById('modal-sms-provider').style.display = 'none';
            await loadSuperSMSProviders();
        } else {
            const data = await res.json();
            showToast(`❌ 保存失败: ${data.error || '未知错误'}`, 'danger');
        }
    } catch (e) {
        showToast('❌ 网络连接错误，保存失败', 'danger');
    }
}

// 物理删除通道数据
window.deleteSMSProvider = async function(id) {
    if (!confirm('⚠️ 危险警告：确定要彻底删除该短信路由通道吗？这将导致对应通道的分流路由彻底失效。')) return;
    
    try {
        const res = await fetch(`/api/admin/sms-providers?id=${encodeURIComponent(id)}`, {
            method: 'DELETE'
        });
        
        if (res.ok) {
            showToast('🗑️ 短信通道已被物理移除成功！', 'success');
            await loadSuperSMSProviders();
        } else {
            const data = await res.json();
            showToast(`❌ 删除失败: ${data.error || '未知错误'}`, 'danger');
        }
    } catch (e) {
        showToast('❌ 网络错误，删除失败', 'danger');
    }
};

// =========================================================================
// 6. 辅助与动效工具 (Toasts & Formatter)
// =========================================================================

// HTML 转义防 XSS
function escapeHTML(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;')
              .replace(/</g, '&lt;')
              .replace(/>/g, '&gt;')
              .replace(/"/g, '&quot;')
              .replace(/'/g, '&#039;');
}

// 时间美化
function formatTime(isoStr) {
    if (!isoStr) return '-';
    try {
        const date = new Date(isoStr);
        return date.toLocaleString('zh-CN', { hour12: false });
    } catch (e) {
        return isoStr;
    }
}

// 全局通知 Toast 控件 (毛玻璃高拟真效果)
function showToast(message, type = 'success') {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    
    const colors = {
        'success': '#10b981',
        'danger': '#ef4444',
        'secondary': '#6b7280'
    };
    
    toast.style.position = 'fixed';
    toast.style.bottom = '20px';
    toast.style.right = '20px';
    toast.style.zIndex = '99999';
    toast.style.background = 'rgba(255, 255, 255, 0.95)';
    toast.style.backdropFilter = 'blur(16px)';
    toast.style.border = `1px solid ${colors[type] || '#cbd5e1'}`;
    toast.style.color = '#1e293b';
    toast.style.padding = '12px 24px';
    toast.style.borderRadius = '12px';
    toast.style.boxShadow = '0 8px 32px 0 rgba(0, 0, 0, 0.08)';
    toast.style.fontFamily = "'Outfit', sans-serif";
    toast.style.fontSize = '14px';
    toast.style.fontWeight = '500';
    toast.style.transition = 'all 0.3s ease';
    toast.style.transform = 'translateY(100px)';
    toast.style.opacity = '0';
    
    toast.innerText = message;
    document.body.appendChild(toast);
    
    // 渲染动效
    setTimeout(() => {
        toast.style.transform = 'translateY(0)';
        toast.style.opacity = '1';
    }, 50);
    
    // 自动淡出
    setTimeout(() => {
        toast.style.transform = 'translateY(100px)';
        toast.style.opacity = '0';
        setTimeout(() => {
            document.body.removeChild(toast);
        }, 300);
    }, 3000);
}

// 复制网关对接 Portal 地址到剪贴板
window.copyPortalUrl = function(hotelId) {
    const portalUrl = `${window.location.protocol}//${window.location.host}/hotel/${hotelId}`;
    
    // 使用标准 Clipboard API
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(portalUrl).then(() => {
            showToast('📋 对接 Portal 地址已成功复制到剪贴板！', 'success');
        }).catch(err => {
            console.error('复制失败:', err);
            fallbackCopyText(portalUrl);
        });
    } else {
        fallbackCopyText(portalUrl);
    }
};

function fallbackCopyText(text) {
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed'; // 避免页面滚动
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
        document.execCommand('copy');
        showToast('📋 对接 Portal 地址已成功复制到剪贴板！', 'success');
    } catch (err) {
        alert('复制失败，请手动选择复制: ' + text);
    }
    document.body.removeChild(textArea);
}

// =========================================================================
// 12. 自助商户账户注册交互逻辑 (由系统提供公共通道发送验证码，不产生扣费)
// =========================================================================

// 自助注册发送验证码
async function executeRegisterSendSMS() {
    const phoneVal = document.getElementById('reg-phone').value.trim();
    const btn = document.getElementById('btn-reg-send-code');
    const errDiv = document.getElementById('reg-error');
    
    if (!/^1[3-9]\d{9}$/.test(phoneVal)) {
        errDiv.innerText = '⚠️ 请输入正确的11位中国手机号码';
        errDiv.style.display = 'block';
        return;
    }
    
    errDiv.style.display = 'none';
    btn.disabled = true;
    btn.innerText = '正在发送...';
    
    try {
        const res = await fetch('/api/admin/register/send-sms', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ phone: phoneVal })
        });
        
        const data = await res.json();
        if (res.ok) {
            showToast('📩 注册验证码已免费发送至您的手机！', 'success');
            // 倒计时冷却 60 秒
            let seconds = 60;
            const timer = setInterval(() => {
                seconds--;
                if (seconds <= 0) {
                    clearInterval(timer);
                    btn.disabled = false;
                    btn.innerText = '获取验证码';
                } else {
                    btn.innerText = `重新获取 (${seconds}s)`;
                }
            }, 1000);
        } else {
            errDiv.innerText = `⚠️ ${data.error || '验证码发送失败'}`;
            errDiv.style.display = 'block';
            btn.disabled = false;
            btn.innerText = '获取验证码';
        }
    } catch (err) {
        errDiv.innerText = '⚠️ 网络发送错误，请重试';
        errDiv.style.display = 'block';
        btn.disabled = false;
        btn.innerText = '获取验证码';
    }
}

// 提交自助注册表单
async function executeRegisterSubmit() {
    const phoneVal = document.getElementById('reg-phone').value.trim();
    const codeVal = document.getElementById('reg-code').value.trim();
    const pwdVal = document.getElementById('reg-password').value;
    const confirmPwdVal = document.getElementById('reg-confirm-password').value;
    const errDiv = document.getElementById('reg-error');
    const btn = document.getElementById('btn-register-submit');
    
    if (!phoneVal || !codeVal || !pwdVal || !confirmPwdVal) {
        errDiv.innerText = '⚠️ 所有表单项均为必填项';
        errDiv.style.display = 'block';
        return;
    }
    
    if (!/^1[3-9]\d{9}$/.test(phoneVal)) {
        errDiv.innerText = '⚠️ 注册手机号格式不正确';
        errDiv.style.display = 'block';
        return;
    }
    
    if (codeVal.length !== 6) {
        errDiv.innerText = '⚠️ 验证码必须是6位数字';
        errDiv.style.display = 'block';
        return;
    }
    
    if (pwdVal.length < 6) {
        errDiv.innerText = '⚠️ 密码长度不能小于6位';
        errDiv.style.display = 'block';
        return;
    }
    
    if (pwdVal !== confirmPwdVal) {
        errDiv.innerText = '⚠️ 两次输入的密码不一致';
        errDiv.style.display = 'block';
        return;
    }
    
    errDiv.style.display = 'none';
    btn.disabled = true;
    btn.innerText = '正在提交注册...';
    
    try {
        const res = await fetch('/api/admin/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                phone: phoneVal,
                code: codeVal,
                password: pwdVal
            })
        });
        
        const data = await res.json();
        if (res.ok) {
            showToast('🎉 自助注册成功！已为您自动登录进入控制台。', 'success');
            // 隐藏注册，显示控制台
            document.getElementById('register-container').style.display = 'none';
            await checkLoginStatus();
        } else {
            errDiv.innerText = `⚠️ ${data.error || '注册失败'}`;
            errDiv.style.display = 'block';
        }
    } catch (e) {
        errDiv.innerText = '⚠️ 网络连接错误，请重试';
        errDiv.style.display = 'block';
    } finally {
        btn.disabled = false;
        btn.innerText = '立即注册并登录';
    }
}

// 物理销户及级联删除数据逻辑 (超级管理员专享)
window.deleteUser = async function(user) {
    if (user === 13703770377) {
        showToast('⚠️ 无法删除超级管理员初始账号！', 'danger');
        return;
    }
    
    if (!confirm(`⚠️ 极度危险警报：确定要彻底物理删除商户账号(${user})吗？\n\n这将同步级联删除该商户旗下的：\n1. 所有酒店网关节点\n2. 访客放行审计日志\n3. 短信扣费详单账单\n4. 财务订购充值对账流水\n\n此物理清理操作完全不可逆，请务必谨慎确认！`)) return;
    
    try {
        const res = await fetch(`/api/admin/users?user=${user}`, {
            method: 'DELETE'
        });
        
        if (res.ok) {
            showToast('🗑️ 该商户账号及其旗下的所有级联数据已彻底清理！', 'success');
            await loadSuperUsers();
        } else {
            const data = await res.json();
            showToast(`❌ 销户失败: ${data.error || '未知原因'}`, 'danger');
        }
    } catch (err) {
        showToast('❌ 网络连接故障，销户失败', 'danger');
    }
};

