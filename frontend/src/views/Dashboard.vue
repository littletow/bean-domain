<script setup>
import { ref, h, computed, reactive, onMounted, nextTick, onUnmounted } from 'vue'
import {
    useMessage, NSpace, NText, NDataTable, NTag, NButton, NSpin,
    NPopconfirm, NInput, NGi, NBadge, NTooltip, NPopover, NA,
    NGrid, NIcon, NStatistic, NProgress, NCard, NDivider,
} from 'naive-ui'

import {
    GlobeOutline, ChatbubblesOutline, PaperPlaneOutline, AlertCircleOutline,
    ShieldCheckmarkOutline, SearchOutline, RefreshOutline, RemoveOutline,
    CheckmarkCircleOutline, TimeOutline
} from '@vicons/ionicons5'


import { Events, Browser } from "@wailsio/runtime";

import {
    GetDashboardStats, GetDomainList, BatchDeleteDomainsByNames,
    BatchRefreshDomains
} from '../../bindings/bean-domain/service/domainservice'


const message = useMessage()

const data = ref([])
const loading = ref(true) // 默认开启加载状态
const fetchDomainData = async () => {
    loading.value = true
    try {
        // 调用 Wails 后端方法
        const result = await GetDomainList()
        console.log(result);
        // 3. 将后端返回的数据赋值给 ref
        // 注意：后端返回的字段名需与前端一致（Go 结构体需设置 json tag）
        data.value = result
    } catch (err) {
        console.error("获取数据失败:", err)
    } finally {
        loading.value = false // 结束获取
    }
}
// 搜索关键词
const searchKeyword = ref('')
// 选中的行 ID
const checkedRowKeys = ref([])

// 状态权重映射
const statusOrder = {
    'error': 1,    // 紧急最靠前
    'expired': 2,
    'warning': 3,  // 警告次之
    'safe': 4,     // 正常
    'pending': 5   // 待处理
};
// 过滤 + 排序后的数据
const displayData = computed(() => {
    let result = [...data.value]

    // 搜索过滤
    if (searchKeyword.value) {
        result = result.filter(item =>
            item.domain.toLowerCase().includes(searchKeyword.value.toLowerCase())
        )
    }

    // 状态排序
    return result.sort((a, b) => statusOrder[a.status] - statusOrder[b.status]);
});

// 处理批量删除
const handleBatchDelete = async () => {
    if (checkedRowKeys.value.length === 0) return

    // 1. 根据选中的 ID (checkedRowKeys) 找到对应的域名字符串
    // const selectedDomains = data.value
    //     .filter(item => checkedRowKeys.value.includes(item.id))
    //     .map(item => item.domain)
    const selectedDomains = checkedRowKeys.value;

    try {
        // 2. 发送域名数组给后端
        await BatchDeleteDomainsByNames(selectedDomains)

        // 3. 后端成功后，同步更新前端 UI 状态
        data.value = data.value.filter(item => !selectedDomains.includes(item.domain))

        // 4. 重置状态
        checkedRowKeys.value = []
        message.success(`成功删除 ${selectedDomains.length} 个域名`)

    } catch (err) {
        message.error('删除失败: ' + err)
    }
}

// 处理全量刷新
const handleRefreshAll = async () => {
    if (checkedRowKeys.value.length === 0) return
    console.log('正在刷新所有域名状态...')
    // 1. 提取域名
    // 直接使用，因为它已经是域名数组了
    const selectedDomains = checkedRowKeys.value;
    console.log('debug,', checkedRowKeys.value);
    // 需要绑定ID
    // const selectedDomains = data.value
    //     .filter(item => checkedRowKeys.value.includes(item.id))
    //     .map(item => item.domain)

    console.log('刷新域名,', selectedDomains);
    try {
        // 2. 调用后端触发异步刷新
        message.info(`已开始刷新 ${selectedDomains.length} 个域名，请稍候...`)
        await BatchRefreshDomains(selectedDomains)
        // 3. 提示已开始，清空选中
        checkedRowKeys.value = []
        // 再次更新面板
        refreshDashboardStats();
    } catch (err) {
        message.error('触发刷新失败: ' + err)
    } finally {

    }
}

const getDaysLeft = (days) => {
    if (days < 0) {
        return {
            type: 'error',
            label: `已过期 ${Math.abs(days)} 天`,
            strong: true
        }
    } else if (days === 0) {
        return {
            type: 'error',
            label: '今日到期',
            strong: true
        }
    } else if (days < 30) {
        return {
            type: 'warning', // 预警用橙色，如果想更显眼也可以用 error
            label: `${days} 天`,
            strong: true
        }
    } else {
        return {
            type: 'default',
            label: `${days} 天`,
            strong: false
        }
    }
}

// 列定义
// const columns = [
//     { type: 'selection' },
//     {
//         title: '序号',
//         key: 'index',
//         width: 60,
//         render: (_, index) => index + 1
//     },
//     { title: '域名', key: 'domain', fixed: 'left' },
//     {
//         title: '当前状态',
//         key: 'status',
//         width: 80,
//         render(row) {
//             const map = {
//                 error: { type: 'error', label: '紧急' },
//                 expired: { type: 'error', label: '已过期' },
//                 warning: { type: 'warning', label: '警告' },
//                 safe: { type: 'success', label: '正常' },
//                 pending: { type: 'default', label: '待处理' },
//             };

//             // 1. 获取配置，如果没有匹配到，使用默认配置
//             const config = map[row.status] || { type: 'default', label: row.status || '未知' };

//             // 2. 渲染时使用安全变量
//             return h(
//                 NTag,
//                 { type: config.type, bordered: false, size: 'small' },
//                 { default: () => config.label }
//             );
//         }
//     },
//     { title: 'SSL证书到期时间', key: 'sslDate', width: 140 },
//     {
//         title: 'SSL剩余天数',
//         key: 'sslRemain',
//         width: 120,
//         // render: (row) => h(NText, { depth: row.sslRemain < 30 ? 1 : 3, type: row.sslRemain < 30 ? 'danger' : '' }, { default: () => `${row.sslRemain} 天` })
//         // render: (row) => {
//         //     const config = getDaysLeft(row.sslRemain)
//         //     if (row.sslRemain <= 0) {
//         //         return h(NSpace, { align: 'center', size: 4 }, {
//         //             default: () => [
//         //                 h(NIcon, { component: AlertCircleOutline, color: '#d03050' }),
//         //                 h(NText, { type: 'error', strong: true }, { default: () => config.label })
//         //             ]
//         //         })
//         //     }
//         //     return h(NText, { type: config.type }, { default: () => config.label })
//         // },
//         render: (row) => {
//             // 检查是否有值，且不是空字符串
//             if (row.sslRemain === undefined || row.sslRemain === null || row.sslRemain === '') {
//                 return h(NText, { depth: 3 }, { default: () => '计算中...' })
//             }
//             const config = getDaysLeft(Number(row.sslRemain))
//             return h(
//                 NText,
//                 {
//                     type: config.type,
//                     strong: config.strong,
//                     // 如果你想针对过期域名加粗显示，可以在这里调整
//                     depth: config.type === 'default' ? 3 : 1
//                 },
//                 { default: () => config.label }
//             )
//         }
//     },
//     { title: '域名到期时间', key: 'domainDate', width: 140 },
//     {
//         title: '域名剩余天数',
//         key: 'domainRemain',
//         width: 120,
//         render: (row) => `${row.domainRemain} 天`
//     }
// ];

const columns = [
    { type: 'selection' },
    {
        title: '序号',
        key: 'index',
        width: 60,
        render: (_, index) => index + 1
    },
    { title: '域名', key: 'domain', fixed: 'left', minWidth: 200 },

    // --- 维度 1：证书状态 ---
    {
        title: '证书状态',
        key: 'sslStatus',
        width: 140,
        render: (row) => renderDimensionStatus(row.sslStatus, row.sslError, row.sslRemain)
    },
    {
        title: '证书到期',
        key: 'sslDate',
        width: 160,
        render: (row) => row.sslDate || h(NText, { depth: 3 }, { default: () => '-' })
    },

    // --- 维度 2：域名状态 ---
    {
        title: '域名状态',
        key: 'domainStatus',
        width: 140,
        render: (row) => renderDimensionStatus(row.domainStatus, row.domainError, row.domainRemain)
    },
    {
        title: '域名到期',
        key: 'domainDate',
        width: 160,
        render: (row) => row.domainDate || h(NText, { depth: 3 }, { default: () => '-' })
    }
];

function renderDimensionStatus(status, errorMsg, remainDays) {
    // 1. 状态映射配置
    const statusMap = {
        valid: { type: 'success', label: '正常', icon: CheckmarkCircleOutline },
        warning: { type: 'warning', label: '预警', icon: TimeOutline },
        expired: { type: 'error', label: '已过期', icon: AlertCircleOutline },
        error: { type: 'error', label: '扫描失败', icon: AlertCircleOutline },
        pending: { type: 'default', label: '待处理', icon: TimeOutline },
    };

    const config = statusMap[status] || { type: 'default', label: status || '未知', icon: TimeOutline };

    // 2. 如果是错误状态，包装 Popover 显示具体报错
    if (status === 'error' && errorMsg) {
        return h(
            NPopover,
            { trigger: 'hover', placement: 'top', style: 'max-width: 300px' },
            {
                trigger: () => h(
                    NTag,
                    { type: 'error', bordered: false, size: 'small', style: 'cursor: help' },
                    {
                        default: () => config.label,
                        icon: () => h(NIcon, { component: config.icon })
                    }
                ),
                default: () => h('div', { style: 'font-size: 12px; color: #d03050' }, errorMsg)
            }
        )
    }

    // 3. 正常/预警/过期状态显示天数提示
    let label = config.label;
    if (status === 'valid' || status === 'warning') {
        label = remainDays > 0 ? `${remainDays}天` : config.label;
    }

    return h(
        NTag,
        { type: config.type, bordered: false, size: 'small' },
        {
            default: () => label,
            icon: () => h(NIcon, { component: config.icon })
        }
    );
}

// 1. 定义仪表盘响应式数据
const stats = reactive({
    currentTotal: 0,
    maxLimit: 100,
    alertCount: 0,
    sslAlertCount: 0,// 证书异常数
    domainAlertCount: 0,//域名异常数
    expiredCount: 0,//已过期总数
    notifyThreshold: 7,
    notifyTime: '--:--',
    lastScanTime: '尚未执行',
    nextScanTime: '等待调度',
    webhookStatus: {
        wechat: false,
        dingtalk: false,
        feishu: false
    },
    // 新增：控制红点显示的标记
    unreadFlags: {
        quota: false,    // 资产容量卡片
        notify: false,   // 通知链路卡片
        schedule: false, // 扫描调度卡片
        alert: false     // 预警资产卡片
    }
});

// 处理红点消失逻辑
const clearBadge = (key, delay = 5000) => {
    setTimeout(() => {
        stats.unreadFlags[key] = false;
    }, delay);
};

// 2. 获取数据的方法
const refreshDashboardStats = async () => {
    try {
        // 调用 Wails 3 后端接口
        const res = await GetDashboardStats();
        console.log('统计面板，', res);
        Object.keys(res.updatedFields).forEach(key => {
            if (res.updatedFields[key]) {
                stats.unreadFlags[key] = true;

                // 2. 如果不需要用户手动点，多长时间后自动消失
                // clearBadge(key, 8000);
            }
        });
        // 直接更新响应式对象
        stats.currentTotal = res.currentTotal;
        stats.maxLimit = res.maxLimit;
        stats.alertCount = res.alertCount;
        stats.domainAlertCount = res.domainAlertCount;
        stats.sslAlertCount = res.sslAlertCount;
        stats.expiredCount = res.expiredCount;
        stats.notifyThreshold = res.notifyThreshold;
        stats.notifyTime = res.notifyTime || '未设置';
        stats.lastScanTime = res.lastScanTime || '无记录';
        stats.nextScanTime = res.nextScanTime || '等待计算';
        stats.webhookStatus = res.webhookStatus;

    } catch (err) {
        console.error("加载仪表盘统计失败:", err);
    }
};


const isScanning = ref(false);
const scanDetail = ref({
    domain: '',
    index: 0,
    total: 0
});

// 3. 生命周期：挂载时加载
onMounted(() => {
    refreshDashboardStats();
    fetchDomainData();
    // 监听配置更新事件
    const off = Events.On("config-updated", async (data) => {
        console.log(data, 'config 更新')
        await nextTick();
        refreshDashboardStats();
    });

    const cleanup = Events.On('domain-refreshed', async (data) => {
        console.log(data, 'domain 更新')
        await nextTick();
        fetchDomainData();
    })

    const scandomain = Events.On('scan-status', async (data) => {
        console.log(data, 'status 更新')
        const { is_scanning, current_domain, current_index, total_count } = data.data;

        isScanning.value = is_scanning;
        scanDetail.value = {
            domain: current_domain,
            index: current_index,
            total: total_count
        };

        console.log(`进度更新: ${current_index}/${total_count} - ${current_domain}`);
    })

    // 卸载时取消监听，防止内存泄漏
    onUnmounted(() => { off(); cleanup(); scandomain() });
});
// 动态调整高度
const tableHeight = ref(window.innerHeight - 380);
window.onresize = () => {
    console.log(tableHeight.value);
    tableHeight.value = window.innerHeight - 380;
};

</script>

<template>
    <div class="dashboard-container">
        <!-- 顶部仪表盘：四栏式布局 -->
        <n-grid :cols="4" :x-gap="12" style="margin-bottom: 20px">
            <!-- 1. 资产配额卡片 -->
            <n-gi>
                <n-badge :show="stats.unreadFlags.quota" value="更新" :offset="[-30, 20]" type="error" style="display: block">
                    <n-card size="small" class="stat-card" @click="stats.unreadFlags.quota = false">
                        <n-statistic label="资产容量">
                            <template #prefix><n-icon color="#18a058">
                                    <GlobeOutline />
                                </n-icon></template>
                            <template #suffix>
                                <n-text depth="3" style="font-size: 14px">/ {{ stats.maxLimit }}</n-text>
                            </template>
                            {{ stats.currentTotal }}
                        </n-statistic>
                        <n-progress type="line" :percentage="Math.round((stats.currentTotal / stats.maxLimit) * 100)"
                            :show-indicator="false" status="success" style="margin-top: 8px" />
                    </n-card>
                </n-badge>
            </n-gi>

            <!-- 2. 推送渠道状态 -->
            <n-gi>
                <n-badge :show="stats.unreadFlags.notify" value="更新" :offset="[-30, 20]" type="error"
                    style="display: block">
                    <n-card size="small" class="stat-card" @click="stats.unreadFlags.notify = false">
                        <n-space vertical :size="4">
                            <n-text depth="3" style="font-size: 12px">通知链路</n-text>
                            <n-space :size="12" style="margin-top: 4px">
                                <n-tooltip trigger="hover">
                                    <template #trigger>
                                        <n-badge :dot="stats.webhookStatus.wechat"
                                            :type="stats.webhookStatus.wechat ? 'success' : 'error'">
                                            <n-icon size="24" :color="stats.webhookStatus.wechat ? '#07C160' : '#ccc'">
                                                <ChatbubblesOutline />
                                            </n-icon>
                                        </n-badge>
                                    </template>
                                    企业微信：{{ stats.webhookStatus.wechat ? '已就绪' : '未配置' }}
                                </n-tooltip>
                                <n-tooltip trigger="hover">
                                    <template #trigger>
                                        <n-badge :dot="stats.webhookStatus.dingtalk"
                                            :type="stats.webhookStatus.dingtalk ? 'success' : 'error'">
                                            <n-icon size="24" :color="stats.webhookStatus.dingtalk ? '#1890FF' : '#ccc'">
                                                <PaperPlaneOutline />
                                            </n-icon>
                                        </n-badge>
                                    </template>
                                    钉钉：{{ stats.webhookStatus.dingtalk ? '已就绪' : '未配置' }}
                                </n-tooltip>
                                <n-tooltip trigger="hover">
                                    <template #trigger>
                                        <n-badge :dot="stats.webhookStatus.feishu"
                                            :type="stats.webhookStatus.feishu ? 'success' : 'error'">
                                            <n-icon size="24" :color="stats.webhookStatus.feishu ? '#3370FF' : '#ccc'">
                                                <PaperPlaneOutline />
                                            </n-icon>
                                        </n-badge>
                                    </template>
                                    飞书：{{ stats.webhookStatus.feishu ? '已就绪' : '未配置' }}
                                </n-tooltip>
                            </n-space>
                            <n-text depth="3" style="font-size: 12px; margin-top: 4px">
                                推送时间：<n-text strong>{{ stats.notifyTime }}</n-text>
                            </n-text>
                            <!-- 底部说明 -->
                            <div>
                                <n-text depth="3" style="font-size: 12px">
                                    通知阈值：{{ stats.notifyThreshold }} 天
                                </n-text>
                            </div>
                        </n-space>
                    </n-card>
                </n-badge>
            </n-gi>

            <!-- 3. 调度周期监控 -->
            <n-gi>
                <n-badge :show="stats.unreadFlags.schedule" value="更新" :offset="[-30, 20]" type="error"
                    style="display: block">
                    <n-card size="small" class="stat-card" @click="stats.unreadFlags.schedule = false">
                        <n-space vertical :size="2">
                            <n-text depth="3" style="font-size: 12px">扫描调度</n-text>
                            <div style="margin-top: 4px">
                                <n-text depth="2" style="font-size: 13px">上次：{{ stats.lastScanTime }}</n-text>
                            </div>
                            <div>
                                <n-text type="info" style="font-size: 13px">下次：{{ stats.nextScanTime }}</n-text>
                            </div>

                        </n-space>
                    </n-card>
                </n-badge>
            </n-gi>

            <!-- 4. 预警状态统计 -->
            <n-gi>
                <n-badge :show="stats.unreadFlags.alert" value="更新" :offset="[-30, 20]" type="error" style="display: block">
                    <n-card size="small" class="stat-card" @click="stats.unreadFlags.alert = false">
                        <n-space vertical :size="2">
                            <!-- 顶部标签 -->
                            <n-text depth="3" style="font-size: 12px">风险资产监控</n-text>

                            <!-- 主数值：总预警数 -->
                            <n-space align="baseline" :size="8" style="margin-top: 2px">
                                <n-text type="error" style="font-size: 24px; font-weight: bold; line-height: 1">
                                    {{ stats.alertCount }}
                                </n-text>
                                <n-text depth="3" style="font-size: 12px">个待处理</n-text>
                            </n-space>

                            <!-- 细分维度：使用小图标或简洁文字 -->
                            <n-space :size="12" style="margin-top: 6px">
                                <n-tooltip trigger="hover">
                                    <template #trigger>
                                        <n-space :size="4" align="center">
                                            <n-icon size="14" :color="stats.sslAlertCount > 0 ? '#d03050' : '#ccc'">
                                                <ShieldCheckmarkOutline />
                                            </n-icon>
                                            <n-text :depth="stats.sslAlertCount > 0 ? 1 : 3" style="font-size: 12px">
                                                证书: {{ stats.sslAlertCount }}
                                            </n-text>
                                        </n-space>
                                    </template>
                                    证书即将到期或扫描异常
                                </n-tooltip>

                                <n-tooltip trigger="hover">
                                    <template #trigger>
                                        <n-space :size="4" align="center">
                                            <n-icon size="14" :color="stats.domainAlertCount > 0 ? '#d03050' : '#ccc'">
                                                <GlobeOutline />
                                            </n-icon>
                                            <n-text :depth="stats.domainAlertCount > 0 ? 1 : 3" style="font-size: 12px">
                                                域名: {{ stats.domainAlertCount }}
                                            </n-text>
                                        </n-space>
                                    </template>
                                    域名即将到期或Whois抓取异常
                                </n-tooltip>
                            </n-space>

                            <!-- 底部过期提示 -->
                            <n-text tag="span" :type="stats.expiredCount > 0 ? 'error' : 'depth'" style="font-size: 11px;">
                                已过期总数：
                                <n-text tag="span" strong :type="stats.expiredCount > 0 ? 'error' : ''">
                                    {{ stats.expiredCount }}
                                </n-text>
                            </n-text>
                        </n-space>
                    </n-card>
                </n-badge>
            </n-gi>
        </n-grid>
        <n-divider style="margin: 12px 0 6px 0" />
        <!-- 原有的操作与表格区，略微优化间距 -->
        <n-card :bordered="false" content-style="padding: 0;">
            <div class="action-bar"
                style="padding: 12px 0; display: flex; justify-content: space-between; align-items: center">
                <n-space>
                    <n-input v-model:value="searchKeyword" placeholder="检索域名..." clearable style="width: 240px">
                        <template #prefix><n-icon>
                                <SearchOutline />
                            </n-icon></template>
                    </n-input>
                    <n-button :disabled="checkedRowKeys.length == 0" @click="handleRefreshAll" secondary type="primary">
                        <template #icon><n-icon>
                                <RefreshOutline />
                            </n-icon></template>
                        扫描
                    </n-button>
                    <n-popconfirm @positive-click="handleBatchDelete">
                        <template #trigger>
                            <n-button :disabled="checkedRowKeys.length == 0" type="error" secondary>
                                <template #icon><n-icon>
                                        <RemoveOutline />
                                    </n-icon></template>
                                删除</n-button>
                        </template>
                        确认从监控清单中移除？
                    </n-popconfirm>
                    <!-- 扫描动画：仅在 isScanning 为 true 时显示 -->
                    <n-space align="center" :size="8" v-if="isScanning" style="margin-left: 12px">
                        <!-- 旋转图标 -->
                        <n-spin size="small" />
                        <!-- 动态文字说明 -->
                        <n-text depth="3" style="font-size: 12px; line-height: 1">
                            正在检测：{{ scanDetail.domain }} ({{ scanDetail.index }}/{{ scanDetail.total }})
                        </n-text>
                    </n-space>
                </n-space>

            </div>
            <n-data-table v-model:checked-row-keys="checkedRowKeys" :columns="columns" :data="displayData"
                :loading="loading" :row-key="(row) => row.domain" :max-height="tableHeight" virtual-scroll />

        </n-card>
    </div>
</template>

<style scoped>
.dashboard-container {
    padding: 8px;
}

.stat-card {
    border-radius: 8px;
    height: 135px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    transition: transform 0.2s, box-shadow 0.2s;
}

.stat-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.alert-bg {
    background: linear-gradient(135deg, #fffcfc 0%, #fff0f0 100%);
}

:deep(.n-statistic .n-statistic-value__content) {
    font-size: 22px;
    font-weight: 600;
}


.action-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: 4px;
}
</style>