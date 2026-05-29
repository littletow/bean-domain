<script setup>
import { ref, h, computed, onMounted, onUnmounted } from 'vue'
import {
  NLayout, NLayoutSider, NLayoutContent, NMenu, NIcon, NTabs, NTabPane, NForm, NFormItem,
  NCard, NSpace, NText, NButton, NBadge, NAlert, NModal, NImage, NInput,
  NConfigProvider, NMessageProvider, NDialogProvider, NNotificationProvider, useMessage,
  zhCN, dateZhCN, darkTheme, createDiscreteApi, NDivider
} from 'naive-ui'


import {
  SpeedometerOutline,   // 监控面板 (替代 Dashboard)
  SettingsOutline,      // 配置 (替代 Setting)
  HelpCircleOutline,    // 帮助 (替代 QuestionCircle)
  SunnyOutline,           // 太阳 (完美支持)
  MoonOutline          // 月亮 (完美支持)
} from '@vicons/ionicons5'

import { Events, Browser } from "@wailsio/runtime";



// 界面视图
import Dashboard from './views/Dashboard.vue'
import Settings from './views/Settings.vue'
import Help from './views/Help.vue'

// 后端绑定方法
import {
  GetAppVersion, GetRemoteVersion,
  IsFirstRun, SetPassword, GetMpCode
} from "../bindings/bean-domain/service/domainservice";


// 主题控制
const isDark = ref(false)

// 菜单配置
const menuOptions = [
  { label: '监控面板', key: 'dashboard', icon: () => h(NIcon, null, { default: () => h(SpeedometerOutline) }) },
  { label: '配置', key: 'settings', icon: () => h(NIcon, null, { default: () => h(SettingsOutline) }) },
  { label: '帮助', key: 'help', icon: () => h(NIcon, null, { default: () => h(HelpCircleOutline) }) }
]

const activeKey = ref('dashboard')


// 建立 key 和 组件的映射关系
const componentMap = {
  dashboard: Dashboard,
  settings: Settings,
  help: Help
}

// 1. 初始化 message，这样不需要 n-message-provider 也能用
const { message } = createDiscreteApi(['message'])


// 计算当前应该显示的组件
const currentComponent = computed(() => componentMap[activeKey.value])

const version = ref('v0.0.0')
const remoteInfo = ref({
  latestVersion: '',
  downloadUrl: '',
  updateLog: '',
})

const showUpdateModal = ref(false)

const hasUpdate = computed(() => {
  if (!remoteInfo.value.latestVersion) return false
  return remoteInfo.value.latestVersion !== version.value
})

const checkRemoteVersion = async () => {
  try {
    const res = await GetRemoteVersion()
    remoteInfo.value = res
  } catch (err) {
    console.error("检查更新失败", err)
  }
}

const checkLocalVersion = async () => {
  try {
    const res = await GetAppVersion()
    version.value = res
  } catch (err) {
    console.error("检查更新失败", err)
  }
}

const copyLink = () => {
  navigator.clipboard.writeText(remoteInfo.value.downloadUrl)
  message.success('下载链接已复制到剪贴板')
}

const showQRCode = ref(false); // 默认不显示二维码

const mpCodeImg = ref('')
const mpCodeTips = ref('使用微信扫码，授权后即可免密使用')
const showPasswordModal = ref(false)
const loading = ref(false)
const passwordMode = ref('manual')
const formRef = ref(null)
const form = ref({
  password: '',
  confirm: ''
})

const rules = {
  password: [
    { required: true, message: '请输入密码' },
    { min: 8, message: '至少 8 位' }
  ],
  confirm: [
    {
      validator: (rule, value) => {
        if (value !== form.value.password) {
          return new Error('两次密码不一致')
        }
        return true
      }
    }
  ]
}


const handleConfirm = async () => {
  loading.value = true
  try {
    if (passwordMode.value === 'manual') {
      await formRef.value?.validate()
      await SetPassword(form.value.password)
    } else {
      // await bindWechatOpenID()
    }
    showPasswordModal.value = false
  } catch (e) {
    message.error(e.message || '设置失败')
  } finally {
    loading.value = false
  }
}

const isFirstRun = async () => {
  try {
    const res = await IsFirstRun()
    showPasswordModal.value = res
    if (res) {
      const res2 = await GetMpCode()
      if (!res2) {
        mpCodeTips.value = '系统错误，请联系作者！'
      } else {
        mpCodeImg.value = res2
      }
    }
  } catch (err) {
    console.error("获取是否首次运行失败", err)
  }
}



onMounted(() => {
  checkLocalVersion()
  checkRemoteVersion()
  isFirstRun()
  //   添加一个扫码成功状态更新
  const mpcodestatus = Events.On('mpcode-status', async (data) => {
    console.log(data, 'status 更新')
    const { status, msg } = data.data;

    if (status == 'ok') {
      showPasswordModal.value = false
    } else {
      message.error(msg)
    }
  })
  onUnmounted(() => { mpcodestatus() });
})
</script>

<template>
  <n-config-provider :theme="isDark ? darkTheme : null" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>

          <n-layout has-sider class="main-container">
            <!-- 左侧边栏 -->
            <n-layout-sider bordered width="256px" content-style="display: flex; flex-direction: column; height: 100vh;">
              <!-- 顶部：图标、标题、主题切换 -->
              <div class="sidebar-header">
                <div class="brand">
                  <!-- 用一个圆角的盒子包裹字母 B -->
                  <div class="b-logo">B</div>
                  <span class="title">豆子域名管家</span>
                </div>
                <n-button quaternary circle @click="isDark = !isDark">
                  <template #icon>
                    <n-icon>
                      <SunnyOutline v-if="isDark" />
                      <MoonOutline v-else />
                    </n-icon>
                  </template>
                </n-button>
              </div>

              <!-- 中间：菜单 -->
              <div style="flex: 1">
                <n-menu v-model:value="activeKey" :options="menuOptions" :indent="20" />
              </div>

              <!-- 底部：卡片信息 -->
              <div class="sidebar-footer">
                <!-- 当有更新时，点击触发弹窗 -->
                <div :style="{ cursor: hasUpdate ? 'pointer' : 'default' }"
                  @click="hasUpdate && (showUpdateModal = true)">
                  <n-badge :dot="hasUpdate" type="error" :offset="[6, 0]">
                    <n-text depth="3" class="version-text">
                      版本：{{ version }}
                    </n-text>
                  </n-badge>
                </div>

              </div>
            </n-layout-sider>

            <!-- 版本升级弹窗小程序版本 -->
            <!-- <n-modal v-model:show="showUpdateModal" preset="card" title="🚀 发现新版本" style="width: 400px">
              <n-space vertical size="large">
               
                <div style="display: flex; justify-content: space-between; align-items: center">
                  <n-text depth="3">当前版本：{{ version }}</n-text>
                  <n-text type="primary" strong>最新版本：{{ remoteInfo.latestVersion }}</n-text>
                </div>

                 
                <template v-if="!showQRCode">
                  <n-divider title-placement="left" style="margin: 12px 0">更新内容</n-divider>
                  <n-text depth="2" style="white-space: pre-wrap; display: block; max-height: 150px; overflow-y: auto;">
                    {{ remoteInfo.updateLog || '暂无详细更新说明' }}
                  </n-text>
                </template>

                <template v-else>
                  <n-divider title-placement="left" style="margin: 12px 0">扫码获取链接</n-divider>
                  <div style="display: flex; flex-direction: column; align-items: center; padding: 10px 0;">
                    <n-image width="180" src="/images/suncode_domain.png" />
                    <n-text depth="3" style="margin-top: 10px; font-size: 12px;">
                      请使用微信扫码，观看视频后即可获取
                    </n-text>
                    <n-button text type="primary" size="tiny" @click="showQRCode = false" style="margin-top: 8px;">
                      返回查看更新说明
                    </n-button>
                  </div>
                </template>

                <n-alert v-if="!showQRCode" title="温馨提示" type="info" :bordered="false">
                  为了维持服务器开销，请扫码观看广告后获取下载地址，感谢支持！
                </n-alert>
              </n-space>

              <template #footer>
                <n-space justify="end">
                  <n-button @click="showUpdateModal = false">稍后再说</n-button>
                   
                  <n-button v-if="!showQRCode" type="primary" @click="showQRCode = true">
                    获取下载链接
                  </n-button>
                  <n-button v-else @click="showQRCode = false">
                    返回
                  </n-button>
                </n-space>
              </template>
            </n-modal> -->

            <!-- 版本升级弹窗 -->
            <n-modal v-model:show="showUpdateModal" preset="card" title="🚀 发现新版本" :closable="false"
              :mask-closable="false" :close-on-esc="false" style="width: 420px">
              <n-space vertical size="large">
                <!-- 版本信息 -->
                <div style="display: flex; justify-content: space-between; align-items: center">
                  <n-text depth="3">当前版本：{{ version }}</n-text>
                  <n-text type="primary" strong>
                    最新版本：{{ remoteInfo.latestVersion }}
                  </n-text>
                </div>

                <n-divider title-placement="left" style="margin: 12px 0">
                  更新内容
                </n-divider>

                <!-- 更新日志 -->
                <n-text depth="2" style="
        white-space: pre-wrap;
        display: block;
        max-height: 180px;
        overflow-y: auto;
      ">
                  {{ remoteInfo.updateLog || '暂无详细更新说明' }}
                </n-text>

                <!-- ✅ 简化提示 -->
                <n-alert title="获取新版本" type="info" :bordered="false">
                  本项目为开源软件，最新版本与源码可在 GitHub 获取。<br />
                  <n-button text type="primary" tag="a" href="https://github.com/littletow/bean-domain/release"
                    target="_blank" rel="noopener">
                    前往 GitHub 下载最新版 →
                  </n-button>
                </n-alert>
              </n-space>

              <template #footer>
                <n-space justify="end">
                  <!-- <n-button @click="showUpdateModal = false">
                    稍后再说
                  </n-button> -->
                  <n-button type="primary" @click="showUpdateModal = false">
                    我知道了
                  </n-button>
                </n-space>
              </template>
            </n-modal>

            <!-- 这是设置密码弹窗，用来加密域名数据 -->
            <n-modal v-model:show="showPasswordModal" preset="card" title="🔐 设置访问密码" :closable="false"
              :mask-closable="false" :close-on-esc="false" style="width: 420px">
              <n-space vertical size="large">
                <!-- 模式切换 -->
                <n-tabs v-model:value="passwordMode" type="segment" animated>
                  <n-tab-pane name="manual" tab="手动设置密码" />
                  <n-tab-pane name="wechat" tab="微信扫码免密" />
                </n-tabs>

                <!-- 手动设置密码 -->
                <template v-if="passwordMode === 'manual'">
                  <n-form ref="formRef" :model="form" :rules="rules">
                    <n-form-item label="密码" path="password">
                      <n-input v-model:value="form.password" type="password" show-password-on="click"
                        placeholder="请输入密码" />
                    </n-form-item>

                    <n-form-item label="确认密码" path="confirm">
                      <n-input v-model:value="form.confirm" type="password" show-password-on="click"
                        placeholder="再次输入密码" />
                    </n-form-item>
                  </n-form>
                  <n-space justify="end">

                    <n-button type="primary" :loading="loading" @click="handleConfirm">
                      确认设置
                    </n-button>
                  </n-space>
                </template>

                <!-- 微信扫码 -->
                <template v-else>
                  <div style="display: flex; flex-direction: column; align-items: center">
                    <n-image width="180" :src="mpCodeImg" />
                    <n-text depth="3" style="margin-top: 10px; font-size: 12px">
                      {{ mpCodeTips }}
                    </n-text>
                  </div>
                </template>
              </n-space>

              <template #footer>
                <n-alert type="info" show-icon :bordered="false">
                  此密码用于加密域名信息配置与调度等数据，<strong>仅设置一次</strong>。<br />
                  • 手动设置密码：<strong>丢失无法恢复</strong><br />
                  • 微信扫码免密：无需记密码<br /><br />
                  设置完成后，下次启动无需再次设置。
                </n-alert>

              </template>
            </n-modal>

            <!-- <n-modal v-model:show="showUpdateModal" preset="card" title="🚀 发现新版本" style="width: 400px"
              :segmented="{ content: 'soft', footer: 'soft' }">
              <n-space vertical size="large">
                <div style="display: flex; justify-content: space-between; align-items: center">
                  <n-text depth="3">当前版本：{{ version }}</n-text>
                  <n-text type="primary" strong>最新版本：{{ remoteInfo.latestVersion }}</n-text>
                </div>

                <n-divider title-placement="left" style="margin: 12px 0">更新内容</n-divider>

                <n-text depth="2" style="white-space: pre-wrap; display: block; max-height: 150px; overflow-y: auto;">
                  {{ remoteInfo.updateLog || '暂无详细更新说明' }}
                </n-text>

                <n-alert title="手动更新指引" type="info" :bordered="false">
                  这是一个单文件应用，请复制下载链接后，在浏览器中打开并下载最新的二进制文件，覆盖旧文件即可。
                </n-alert>
              </n-space>

              <template #footer>
                <n-space justify="end">
                  <n-button @click="showUpdateModal = false">稍后再说</n-button>
                  <n-button type="primary" @click="copyLink">
                    复制下载链接
                  </n-button>
                </n-space>
                <n-space justify="end">
                  <n-button @click="showUpdateModal = false">稍后再说</n-button>
                  <n-popover trigger="click" placement="top">
                    <template #trigger>
                      <n-button type="primary">扫码获取更新</n-button>
                    </template>
                    <div style="text-align: center; padding: 10px;">
                      <img src="/images/suncode_domain.png" style="width: 150px; height: 150px;" />
                      <div style="margin-top: 8px; font-size: 12px; color: #666;">
                        扫码看广告，获取高速下载地址
                      </div>
                    </div>
                  </n-popover>
                </n-space>
              </template>
            </n-modal> -->

            <!-- 右侧内容区 (75%) -->
            <n-layout-content content-style="padding: 24px;">
              <keep-alive>
                <component :is="currentComponent" />
              </keep-alive>
            </n-layout-content>
          </n-layout>
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<style scoped>
.main-container {
  height: 100vh;
}

.sidebar-header {
  padding: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid rgba(128, 128, 128, 0.1);
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
}

.b-logo {
  width: 28px;
  height: 28px;
  background-color: #f0a020;
  /* 漂亮的明黄色 */
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 900;
  font-size: 20px;
  border-radius: 6px;
  /* 圆角效果 */
  margin-right: 8px;
  font-family: "Arial Black", Gadget, sans-serif;
  /* 使字体更粗犷 */
  box-shadow: 0 0 8px rgba(240, 160, 32, 0.4);
}

.title {
  font-size: 16px;
  font-weight: bold;
}


.sidebar-footer {
  margin-top: auto;
  padding: 24px 16px;
  /* 增加上下内边距 (从16px增加到24px) */
  border-top: 1px solid rgba(128, 128, 128, 0.1);
  display: flex;
  justify-content: center;
  align-items: center;
  transition: background-color 0.2s;
  /* 增加一个悬停反馈 */
}

.sidebar-footer:hover {
  background-color: rgba(128, 128, 128, 0.05);
}

/* 2. 调大版本号文字 */
.version-text {
  font-size: 14px;
  font-weight: 500;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.info-item {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}
</style>