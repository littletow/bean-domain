<script setup>
import { ref, computed, reactive, onMounted, nextTick } from 'vue'
import {
    NForm, NFormItem, NInput, NButton, NCard, NDivider, NUpload, NUploadDragger,
    NIcon, NText, NTimePicker, NSpace, NSwitch, useMessage, NSteps, NStep,
    NAlert, NGrid, NGi, NTag, NTooltip, NModal, NTab, NTabs, NScrollbar, NTabPane,
    NStatistic, NInputGroup, NInputNumber, NInputGroupLabel
} from 'naive-ui'
import {
    ArchiveOutline as ArchiveIcon,
    SendOutline,
    FlashOutline,
    DocumentTextOutline,
    NotificationsCircleOutline,
    ShieldCheckmarkOutline,
    TimeOutline,
    SearchOutline,
    DownloadOutline,
    EyeOutline,
    KeyOutline,
    InformationCircleOutline,
    PaperPlaneOutline,
    ChatbubblesOutline,
    NotificationsOutline,
    ChatboxEllipsesOutline
} from '@vicons/ionicons5'


import {
    DownloadDomainTemplate, SaveImportedDomains,
    SelectDomainFile, VerifyDomainFile,
    TestWebhook, SaveWebhook,
    SaveScheduleConfig, SetAutoStart,
    CheckAutoStartStatus,
} from '../../bindings/bean-domain/service/domainservice'

const message = useMessage()

// --- 报表预览逻辑 (Markdown) ---
const reportMarkdownPreview = computed(() => {
    return `### 📊 域名监控日报 (2026-01-22)
---
✅ **运行正常**: 120 个
⚠️ **即将到期**: 5 个
❌ **已经过期**: 3 个

**待处理告警域名列表:**
1. \`test-expired.com\`
2. \`demo-old.net\`
3. \`api-v1.io\`

> 豆子域名管家提醒您，请及时处理告警域名。
`
})

const reportPreview = computed(() => {
    return `📊 域名监控日报 (2026-01-22)
━━━━━━━━━━━━━━━━━━━━━━

【运行状态概览】
✅ 运行正常：90 个
⚠️ 即将到期：5 个
❌ 已经过期：3 个

🚨 待处理告警域名列表：
1. test-expired.com
2. demo-old.net
3. api-v1.io

━━━━━━━━━━━━━━━━━━━━━━
💡 豆子域名管家提醒您，请及时处理告警域名。
数据更新于 15:04`
})



const showVerifyModal = ref(false)
const importing = ref(false)
const selectedFilePath = ref('')
// 模拟校验数据结构
const verifyResult = reactive({
    valid: [],
    duplicate: [],
    invalid: [],
    currentTotal: 0, // 数据库已存在数量
    remainingQuota: 20,
})

const isOverLimit = computed(() => verifyResult.valid.length > verifyResult.remainingQuota)

const handleSelectFile = async () => {
    try {
        const path = await SelectDomainFile();
        // const path = "C:\\A.txt";

        if (path) {
            selectedFilePath.value = path;
            message.success("文件路径获取成功");
        }
    } catch (err) {
        console.error("调用后端失败：", err);
        message.error("系统对话框调用异常");
    }
}


const triggerVerify = async () => {
    if (!selectedFilePath.value) return;

    // 1. 开启 Loading 提示
    const loadingMsg = message.loading("正在解析本地文件...", { duration: 0 });

    try {
        // 2. 调用真正的 Wails 3 后端方法
        // 注意：Wails 会自动处理 Go 结构体到 JS 对象的转换
        const result = await VerifyDomainFile(selectedFilePath.value);

        // 3. 将后端返回的真实数据赋值给响应式变量
        verifyResult.valid = result.valid || [];
        verifyResult.duplicate = result.duplicate || [];
        verifyResult.invalid = result.invalid || [];
        verifyResult.currentTotal = result.currentTotal || 0;
        verifyResult.remainingQuota = result.remainingQuota || 20;
        // 4. 弹出确认模态框
        showVerifyModal.value = true;
    } catch (err) {
        // 处理读取文件权限、路径错误等异常
        message.error("校验失败: " + err);
    } finally {
        // 5. 销毁 Loading
        loadingMsg.destroy();
    }
}

// 真正执行导入的操作
const confirmImport = async () => {
    importing.value = true
    try {
        const dataToSave = verifyResult.valid.slice(0, verifyResult.remainingQuota)
        await SaveImportedDomains([...dataToSave]) // 展开运算符断开引用
        console.log("Go 方法执行完毕");
        showVerifyModal.value = false;
        await nextTick();
        message.success(`成功导入 ${verifyResult.valid.length} 条记录`)
    } catch (e) {
        console.error(e) // 打印具体错误到 Wails 控制台
        message.error(e.message || '导入失败')
    } finally {
        importing.value = false
        // 双重保障
        showVerifyModal.value = false
    }
}

const handleDownloadTemplate = async () => {
    try {
        // 调用后端方法
        await DownloadDomainTemplate();
        // 如果后端返回错误会进入 catch
        // 这里不需要额外的成功提示，因为用户在系统对话框点击保存已经有反馈了
    } catch (err) {
        message.error("下载失败: " + err);
    }
};

// Webhook配置
const showPreviewModal = ref(false)
const webhookConfig = reactive({
    wechat: {
        url: ''
    },
    dingtalk: {
        url: '',
        secret: ''
    },
    feishu: {
        url: '',
        secret: ''
    }
});

/**
 * 验证 Webhook：调用后端发送一条测试消息
 */
const testWebhook = async (platform) => {
    const config = webhookConfig[platform];
    if (!config.url) return message.warning("请填写 Webhook 地址");

    const loading = message.loading(`正在测试${platform}...`, { duration: 0 });
    try {
        // 直接传递整个对象，Wails 会自动映射到 Go 的结构体
        await TestWebhook(platform, config);
        message.success("测试消息已发送，请查看群聊");
    } catch (err) {
        message.error("测试失败: " + err);
    } finally {
        loading.destroy();
    }
}

/**
 * 保存单个配置
 */
const saveWebhook = async (platform) => {
    const config = webhookConfig[platform];
    try {
        await SaveWebhook(platform, config);
        message.success("配置已永久保存");
    } catch (err) {
        message.error("保存失败: " + err);
    }
}

// 调度和通知策略
const scheduleConfig = reactive({
    notifyTime: '10:00',
    notifyThreshold: 30,
    enableNotify: true,
});

/**
 * 保存通知策略配置：调用后端服务
 */
const saveScheduleConfig = async () => {
    try {
        // 调用 Wails 3 后端服务保存配置
        await SaveScheduleConfig(scheduleConfig);
        message.success("通知策略配置已保存");
    } catch (err) {
        message.error("保存失败: " + err);
    }
};

const autoStartEnabled = ref(false)
const doSetAutoStart = async () => {
    try {
        await SetAutoStart(autoStartEnabled.value);
        message.success(autoStartEnabled.value ? '已开启开机自启' : '已关闭开机自启')
    } catch (err) {
        message.error("操作失败: " + err);
        autoStartEnabled.value = !autoStartEnabled.value
    }
}

// 页面初始化时同步状态
onMounted(async () => {
    try {
        const isEnabled = await CheckAutoStartStatus()
        autoStartEnabled.value = isEnabled
    } catch (err) {
        console.error("无法获取启动状态:", err)
    }
})

</script>

<template>
    <div class="settings-container">
        <!-- 1. 顶部说明区：仅作为流程指引和状态概览 -->
        <n-card :bordered="false" class="header-intro-card">
            <n-space vertical size="large">
                <!-- 标题区 -->
                <n-space align="center" :size="12">
                    <div class="icon-badge">
                        <n-icon size="22">
                            <FlashOutline />
                        </n-icon>
                    </div>
                    <n-text strong style="font-size: 20px">配置中心</n-text>
                    <n-tag :bordered="false" type="info" size="small" round>v2026.1</n-tag>
                </n-space>

                <!-- 描述区 -->
                <div class="guide-content">
                    <!-- 移除深度 depth="2"，直接用 text 配合样式 -->
                    <n-text class="description-text">
                        欢迎进入资产管理核心。为了确保您的域名证书监控闭环且能够高效运行，请按以下指引操作：
                    </n-text>

                    <n-grid :cols="3" x-gap="24" style="margin-top: 16px">
                        <n-gi>
                            <n-space vertical :size="4">
                                <n-text strong><n-text type="primary">01.</n-text> 数据导入</n-text>
                                <n-text class="sub-text">
                                    上传 TXT 格式资产文件，系统将自动完成域名合规校验与去重存入。
                                </n-text>
                            </n-space>
                        </n-gi>
                        <n-gi>
                            <n-space vertical :size="4">
                                <n-text strong><n-text type="primary">02.</n-text> 联动通知</n-text>
                                <n-text class="sub-text">
                                    配置企业微信或钉钉或飞书 Webhook，打通关键资产到期的即时推送链路。
                                </n-text>
                            </n-space>
                        </n-gi>
                        <n-gi>
                            <n-space vertical :size="4">
                                <n-text strong><n-text type="primary">03.</n-text> 自动化调度</n-text>
                                <n-text class="sub-text">
                                    设定每日扫描时刻与预警阈值，实现资产健康状态的 7×24 自动监管。
                                </n-text>
                            </n-space>
                        </n-gi>
                    </n-grid>
                </div>
            </n-space>
        </n-card>

        <n-space vertical :size="20" style="margin-top: 20px">

            <!-- 模块 1: 导入 (独立操作) -->
            <n-card title="域名资产导入" size="small" :segmented="{ content: true, footer: true }">
                <template #header-extra>
                    <n-space>
                        <n-tag :bordered="false" type="info" size="small">支持追加导入</n-tag>
                    </n-space>
                </template>
                <n-space vertical :size="16">
                    <!-- 提示文字添加底部间距 -->
                    <n-text depth="3" style="font-size: 13px; display: block">
                        请指派包含域名资产的 TXT 文件路径（每行一个域名）：
                    </n-text>

                    <!-- 紧凑型输入组：通过样式微调间隙 -->
                    <div style="display: flex; align-items: center; gap: 12px">
                        <!-- 路径显示区：占用主要空间 -->
                        <n-input v-model:value="selectedFilePath" placeholder="请点击浏览选择本地 TXT 文件..." style="flex: 1"
                            readonly>
                            <template #prefix>
                                <n-icon :component="DocumentTextOutline" />
                            </template>
                        </n-input>

                        <!-- 浏览按钮：添加 margin 间隙 -->
                        <n-button strong secondary type="info" @click="handleSelectFile">
                            <template #icon>
                                <n-icon :component="SearchOutline" />
                            </template>
                            浏览
                        </n-button>

                        <!-- 校验按钮 -->
                        <n-button strong secondary type="primary" :disabled="!selectedFilePath" @click="triggerVerify">
                            <template #icon>
                                <n-icon :component="ShieldCheckmarkOutline" />
                            </template>
                            开始校验
                        </n-button>
                    </div>

                </n-space>
                <template #footer>
                    <n-space align="center" justify="space-between">
                        <n-text depth="3">提示：导入前系统会自动校验并要求你确认，无需担心重复记录。</n-text>
                        <n-button tertiary type="primary" size="small" @click="handleDownloadTemplate">
                            <template #icon><n-icon>
                                    <DownloadOutline />
                                </n-icon></template>
                            下载示例模板
                        </n-button>
                    </n-space>
                </template>
            </n-card>

            <!-- 模块 2: 通知 (独立配置) -->
            <n-card title="远程通知配置" size="small" :segmented="{ content: true }">
                <template #header-extra>
                    <n-button tertiary type="primary" size="small" @click="showPreviewModal = true">
                        <template #icon><n-icon>
                                <EyeOutline />
                            </n-icon></template>
                        推送效果预览
                    </n-button>
                </template>

                <n-space vertical :size="24">
                    <!-- 渠道 1: 企业微信 -->
                    <div class="webhook-row">
                        <div class="channel-info">
                            <n-icon size="24" color="#07C160">
                                <ChatbubblesOutline />
                            </n-icon>
                            <n-text strong>企业微信</n-text>
                        </div>

                        <n-input-group style="flex: 1; margin: 0 12px">
                            <n-input v-model:value="webhookConfig.wechat.url" placeholder="请输入企业微信机器人 Webhook URL..." />
                            <n-button type="info" ghost @click="testWebhook('wechat')">
                                验证测试
                            </n-button>
                            <n-button strong secondary type="primary" @click="saveWebhook('wechat')">
                                保存
                            </n-button>
                        </n-input-group>
                    </div>

                    <!-- 渠道 2: 钉钉 -->
                    <div class="webhook-row">
                        <div class="channel-info">
                            <n-icon size="24" color="#1890FF">
                                <PaperPlaneOutline />
                            </n-icon>
                            <n-text strong>钉钉 (Ding)</n-text>
                        </div>

                        <div style="flex: 1; display: flex; flex-direction: column; gap: 8px; margin: 0 12px">
                            <n-input-group>
                                <n-input v-model:value="webhookConfig.dingtalk.url"
                                    placeholder="Webhook URL (包含 AccessToken)" />
                                <n-button type="info" ghost @click="testWebhook('dingtalk')">
                                    验证测试
                                </n-button>
                                <n-button strong secondary type="primary" @click="saveWebhook('dingtalk')">
                                    保存
                                </n-button>
                            </n-input-group>
                            <n-input v-model:value="webhookConfig.dingtalk.secret" type="password"
                                show-password-on="mousedown" placeholder="加签密钥 (Secret) - 若未开启加签请留空" size="small">
                                <template #prefix>
                                    <n-icon :component="KeyOutline" />
                                </template>
                            </n-input>
                        </div>
                    </div>
                    <!-- 渠道 3: 飞书 -->
                    <div class="webhook-row">
                        <div class="channel-info">
                            <n-icon size="24" color="#3370FF">
                                <PaperPlaneOutline />
                            </n-icon>
                            <n-text strong>飞书</n-text>
                        </div>

                        <div style="flex: 1; display: flex; flex-direction: column; gap: 8px; margin: 0 12px">
                            <n-input-group>
                                <n-input v-model:value="webhookConfig.feishu.url"
                                    placeholder="Webhook URL (包含 AccessToken)" />
                                <n-button type="info" ghost @click="testWebhook('feishu')">
                                    验证测试
                                </n-button>
                                <n-button strong secondary type="primary" @click="saveWebhook('feishu')">
                                    保存
                                </n-button>
                            </n-input-group>
                            <n-input v-model:value="webhookConfig.feishu.secret" type="password"
                                show-password-on="mousedown" placeholder="加签密钥 (Secret) - 若未开启加签请留空" size="small">
                                <template #prefix>
                                    <n-icon :component="KeyOutline" />
                                </template>
                            </n-input>
                        </div>
                    </div>

                    <n-alert :show-icon="false" type="info" style="font-size: 12px">
                        <n-text depth="3">
                            安全提示：建议在机器人配置中勾选「IP地址段」过滤以增强安全性。
                        </n-text>
                    </n-alert>
                </n-space>
            </n-card>

            <!-- 模块 3: 调度与通知策略 -->
            <n-card title="通知策略" size="small" :segmented="{ content: true }">
                <template #header-extra>
                    <n-button strong secondary type="primary" @click="saveScheduleConfig">
                        保存当前策略
                    </n-button>
                </template>

                <n-space vertical :size="20">
                    <!-- 策略行 1: 机器人推送开关 -->
                    <div class="policy-row">
                        <div class="policy-label">
                            <n-icon size="20" color="#18a058">
                                <NotificationsOutline />
                            </n-icon>
                            <n-text strong>机器人自动推送</n-text>
                        </div>
                        <div class="policy-control">
                            <n-space>


                                <n-switch v-model:value="scheduleConfig.enableNotify" />
                            </n-space>
                            <n-text depth="3" style="margin-left: 12px">
                                {{ scheduleConfig.enableNotify ? '开启：系统将根据设定的时间自动推送报告' : '关闭：系统仅静默监控，不发送消息' }}
                            </n-text>
                        </div>

                    </div>

                    <!-- 策略行 2: 每日推送时间 -->
                    <div class="policy-row">
                        <div class="policy-label">
                            <n-icon size="20" color="#f0a020">
                                <TimeOutline />
                            </n-icon>
                            <n-text strong>每日推送时间</n-text>
                        </div>
                        <div class="policy-control">
                            <n-space>
                                <n-time-picker v-model:formatted-value="scheduleConfig.notifyTime" value-format="HH:mm"
                                    format="HH:mm" placeholder="请选择时间" :disabled="!scheduleConfig.enableNotify"
                                    style="width: 140px" />
                            </n-space>
                            <n-text depth="3" style="margin-left: 12px">精确到分钟 (例如 10:00)</n-text>
                        </div>

                    </div>

                    <!-- 策略行 3: 预警阈值 -->
                    <div class="policy-row">
                        <div class="policy-label">
                            <n-icon size="20" color="#d03050">
                                <ShieldCheckmarkOutline />
                            </n-icon>
                            <n-text strong>域名预警阈值</n-text>
                        </div>
                        <div class="policy-control">
                            <n-space>
                                <n-input-group>
                                    <n-input-number v-model:value="scheduleConfig.notifyThreshold" :min="1" :max="365"
                                        placeholder="天数" :disabled="!scheduleConfig.enableNotify" style="width: 100px" />
                                    <n-input-group-label>天</n-input-group-label>
                                </n-input-group>
                            </n-space>
                            <n-text depth="3" style="margin-left: 12px">剩余天数低于此值将触发红色预警推送</n-text>
                        </div>

                    </div>
                </n-space>
            </n-card>
        </n-space>

        <!-- 随系统启动 -->
        <n-space vertical :size="20" style="margin-top: 20px">
            <n-card title="系统设置" size="small" :segmented="{ content: true }">
                <div style="display: flex; align-items: center; justify-content: space-between;">
                    <div style="display: flex; align-items: center; gap: 12px">
                        <n-text depth="3">开机自动启动</n-text>
                        <n-switch v-model:value="autoStartEnabled" />
                    </div>
                    <n-button type="primary" size="small" @click="doSetAutoStart" :loading="loading">
                        保存配置
                    </n-button>
                </div>
            </n-card>
        </n-space>

    </div>
    <!-- 校验结果预览模态框 -->
    <n-modal v-model:show="showVerifyModal" preset="card" title="数据校验报告" style="width: 600px"
        :segmented="{ content: true, footer: true }">
        <n-alert v-if="verifyResult.valid.length > verifyResult.remainingQuota" type="warning" closable
            style="margin-bottom: 16px">
            检测到导入数量（{{ verifyResult.valid.length }}）已超过剩余配额（{{ verifyResult.remainingQuota }}）。
            <template #header>
                配额不足提醒
            </template>
            系统将仅导入前 {{ verifyResult.remainingQuota }} 个域名。您可以先删除旧资产，或联系获取无限制版本。
        </n-alert>
        <n-grid :cols="4" :x-gap="12" :y-gap="12" style="margin-bottom: 20px">
            <n-gi>
                <n-card embedded :bordered="false" size="small">
                    <n-statistic label="检测到有效" :value="verifyResult.valid.length" />
                </n-card>
            </n-gi>
            <n-gi>
                <n-card embedded :bordered="false" size="small">
                    <n-statistic label="剩余配额" :value="verifyResult.remainingQuota" />
                </n-card>
            </n-gi>
            <n-gi>
                <n-card embedded :bordered="false" size="small">
                    <n-statistic label="可导入域名" :value="verifyResult.valid.length" />
                </n-card>
            </n-gi>
            <n-gi>
                <n-card embedded :bordered="false" size="small">
                    <n-statistic label="重复/无效" :value="verifyResult.invalid.length + verifyResult.duplicate.length" />
                </n-card>
            </n-gi>
        </n-grid>

        <div style="margin-top: 20px">
            <n-tabs type="line" :animated="false">
                <n-tab-pane name="valid" :tab="'待导入 (' + verifyResult.valid.length + ')'">
                    <n-scrollbar style="max-height: 200px">
                        <n-tag v-for="(item, index) in verifyResult.valid" :key="'valid-' + index" size="small"
                            type="success" style="margin: 4px">
                            {{ item }}
                        </n-tag>
                    </n-scrollbar>
                </n-tab-pane>
                <n-tab-pane name="invalid"
                    :tab="'异常项 (' + (verifyResult.invalid.length + verifyResult.duplicate.length) + ')'">
                    <n-scrollbar style="max-height: 200px">
                        <n-space vertical size="small">
                            <n-text type="warning" v-for="(item, index) in verifyResult.duplicate" :key="'dup-' + index">
                                [重复] {{ item }}
                            </n-text>
                            <n-text type="error" v-for="(item, index) in verifyResult.invalid" :key="'inv-' + index">
                                [无效] {{ item }}
                            </n-text>
                        </n-space>
                    </n-scrollbar>
                </n-tab-pane>
            </n-tabs>
        </div>

        <!-- 超额警告 -->
        <n-alert v-if="isOverLimit" type="warning" style="margin-top: 15px">
            当前总量将超过系统限额，仅会为您导入前 {{ verifyResult.remainQuota }} 条记录。
        </n-alert>

        <template #footer>
            <n-space justify="end">
                <n-button @click="showVerifyModal = false" :disabled="importing">取消</n-button>
                <n-button type="primary" :loading="importing" :disabled="verifyResult.valid.length === 0"
                    @click="confirmImport">
                    {{ verifyResult.valid.length > verifyResult.remainingQuota ? '截断并导入' : '确认导入' }}
                </n-button>
            </n-space>
        </template>
    </n-modal>

    <!-- 推送效果预览弹窗 -->
    <n-modal v-model:show="showPreviewModal" preset="card" style="width: 450px; border-radius: 12px;" title="消息推送样式预览">
        <div class="preview-container">
            <div class="mock-device-header">
                <n-tag size="small" round :bordered="false" type="info">通知预览</n-tag>
            </div>
            <div class="bot-msg-wrapper">
                <div class="bot-avatar">
                    <n-icon size="20" color="#fff">
                        <InformationCircleOutline />
                    </n-icon>
                </div>
                <div class="msg-bubble">
                    <div class="msg-header">域名监控助手</div>
                    <n-scrollbar style="max-height: 260px">
                        <!-- 展示 2026 年的模拟数据 -->
                        <pre class="markdown-preview">{{ reportPreview }}</pre>
                    </n-scrollbar>
                    <div class="msg-footer"></div>
                </div>
            </div>
        </div>
        <template #footer>
            <n-text depth="3" style="font-size: 12px">
                * 实际显示效果会根据各客户端（企业微信/钉钉/飞书）微调。
            </n-text>
        </template>
    </n-modal>
</template>

<style scoped>
/* 保持你原有的预览窗口样式，以下为补充样式 */
.preview-container {
    background-color: #f5f5f5;
    padding: 20px;
    border-radius: 8px;
}

.bot-msg-wrapper {
    display: flex;
    gap: 12px;
}

.bot-avatar {
    width: 36px;
    height: 36px;
    background-color: #18a058;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.msg-bubble {
    background: #fff;
    padding: 12px;
    border-radius: 4px;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
    flex: 1;
}

.msg-header {
    font-weight: bold;
    margin-bottom: 8px;
}

.webhook-row {
    display: flex;
    align-items: flex-start;
    /* 兼容钉钉的双行输入 */
    padding: 8px 0;
}

.channel-info {
    width: 120px;
    display: flex;
    align-items: center;
    gap: 12px;
    padding-top: 5px;
    /* 对齐输入框高度 */
}

.policy-row {
    display: flex;
    align-items: center;
    padding: 4px 0;
}

.policy-label {
    width: 160px;
    display: flex;
    align-items: center;
    gap: 10px;
}

.policy-control {
    flex: 1;
    display: flex;
    align-items: center;
}

.guide-content {
    background-color: var(--n-action-color);
    padding: 16px;
    border-radius: 8px;
    border: 1px solid var(--n-border-color);
    /* 在深色模式下提供一点点深度感 */
    box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
}

.header-intro-card {
    /* 确保卡片背景在两种模式下都正确 */
    background-color: var(--n-color);
}

.icon-badge {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border-radius: 8px;

    /* 核心修改：背景使用主色调的淡化版（0.15透明度） */
    background-color: var(--n-primary-color-mask);
    /* 或者 rgba(var(--n-primary-color-rgb), 0.15) */

    /* 核心修改：图标颜色直接使用主色调，确保在任何背景下都能看清 */
    color: var(--n-primary-color);

    /* 如果你想保持原本那种深色块+白图标的设计，请看下面的方案 2 */
}

.description-text {
    font-size: 14px;
    line-height: 1.8;
    /* 使用预设的次级文本颜色变量 */
    color: var(--n-text-color);
    opacity: 0.85;
}

.sub-text {
    font-size: 12px;
    /* 使用预设的更淡的文本颜色变量 */
    color: var(--n-text-color);
    opacity: 0.6;
}

/* 针对 N-Grid 中的内容进行微调 */
:deep(.n-text--strong-text) {
    color: var(--n-text-color);
}

.preview-container {
    background-color: var(--n-code-color);
    /* 自动适配主题的背景色 */
    padding: 20px;
    border-radius: 8px;
    border: 1px solid var(--n-border-color);
}

.mock-device-header {
    display: flex;
    justify-content: center;
    margin-bottom: 16px;
    opacity: 0.8;
}

.bot-msg-wrapper {
    display: flex;
    gap: 12px;
}

/* 机器人头像 */
.bot-avatar {
    width: 36px;
    height: 36px;
    background: linear-gradient(135deg, #18a058 0%, #36ad6a 100%);
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    box-shadow: 0 2px 8px rgba(24, 160, 88, 0.3);
}

/* 气泡样式：核心适配点 */
.msg-bubble {
    flex-grow: 1;
    background-color: var(--n-color);
    /* 适配卡片背景 */
    border: 1px solid var(--n-border-color);
    border-radius: 0 12px 12px 12px;
    padding: 12px;
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
    position: relative;
}

.msg-header {
    font-weight: 600;
    font-size: 14px;
    margin-bottom: 8px;
    color: var(--n-text-color);
}

.markdown-preview {
    white-space: pre-wrap;
    word-wrap: break-word;
    font-family: "SF Mono", Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
    font-size: 13px;
    line-height: 1.6;
    margin: 0;
    color: var(--n-text-color);
    background-color: transparent;
    /* 移除默认 pre 背景 */
}

.msg-footer {
    font-size: 11px;
    color: var(--n-text-color-3);
    /* 使用中性文本色 */
    margin-top: 10px;
    border-top: 1px dashed var(--n-border-color);
    /* 改为虚线更具现代感 */
    padding-top: 6px;
    display: flex;
    justify-content: space-between;
}

/* 针对暗黑模式的微调 */
[data-theme='dark'] .msg-bubble {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}
</style>