<script setup>
import { ref } from 'vue'
import {
    ShieldCheckmarkOutline,
    LockClosedOutline,
    MailOutline,
    BookOutline,
    HelpCircleOutline
} from '@vicons/ionicons5'

import {
    NSpace, NButton, NDivider, NIcon,
} from 'naive-ui'

// 1. 修改为本地路径
const HOME_URL = '/faq.html' // 对应 public/faq.html
const helpUrl = ref(HOME_URL)
// 2. 切换 URL 的方法
const setUrl = (url) => {
    helpUrl.value = url
}

// 3. 系统浏览器打开特定链接，可以使用 Wails 原生方法
import { Browser } from "@wailsio/runtime";
const openExternal = (url) => Browser.OpenURL(url)
</script>

<template>
    <div class="help-container">
        <div class="doc-viewer">
            <iframe :key="helpUrl" :src="helpUrl" frameborder="0" class="help-iframe"></iframe>
        </div>

        <n-divider title-placement="left">快速操作</n-divider>

        <div class="footer-actions">
            <n-space justify="space-around" align="center">
                <!-- 内部本地页面 -->
                <n-button :type="helpUrl === HOME_URL ? 'primary' : 'default'" quaternary @click="setUrl(HOME_URL)">
                    <template #icon><n-icon>
                            <HelpCircleOutline />
                        </n-icon></template>
                    帮助中心
                </n-button>

                <n-button :type="helpUrl === '/disclaimer.html' ? 'primary' : 'default'" quaternary
                    @click="setUrl('/disclaimer.html')">
                    <template #icon><n-icon>
                            <ShieldCheckmarkOutline />
                        </n-icon></template>
                    免责声明
                </n-button>

                <n-button :type="helpUrl === '/privacy.html' ? 'primary' : 'default'" quaternary
                    @click="setUrl('/privacy.html')">
                    <template #icon><n-icon>
                            <LockClosedOutline />
                        </n-icon></template>
                    隐私政策
                </n-button>

                <!-- 外部链接建议：如果是联系作者或教程，建议调用系统默认浏览器打开，而不是在 iframe 里 -->
                <n-button quaternary @click="setUrl('/contact.html')">
                    <template #icon><n-icon>
                            <MailOutline />
                        </n-icon></template>
                    联系作者
                </n-button>

                <!-- <n-button type="primary" dashed @click="openExternal('https://91demo.top')">
                    <template #icon><n-icon>
                            <BookOutline />
                        </n-icon></template>
                    更多教程
                </n-button> -->
            </n-space>
        </div>
    </div>
</template>

<style scoped>
.help-container {
    display: flex;
    flex-direction: column;
    /* 关键：强制高度为窗口高度减去外层可能存在的 padding */
    height: calc(100vh - 96px);
    padding: 16px;
    /* 关键：禁止外层出现滚动条 */
    overflow: hidden;
}

.doc-viewer {
    flex: 1;
    /* 撑开剩余空间 */
    display: flex;
    /* 确保内部 iframe 填充 */
    background-color: var(--n-action-color);
    border-radius: 8px;
    border: 1px solid var(--n-border-color);
    /* 关键：确保内部内容溢出时不撑大父容器 */
    min-height: 0;
}

.help-iframe {
    width: 100%;
    height: 100%;
    border: none;
    display: block;
    /* 消除行内元素底部的微小空隙 */
}

.footer-actions {
    margin-top: 8px;
}
</style>