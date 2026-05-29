# 🌐 Bean Domain

**BeanDomain** 是一款基于 **Wails v2** 开发的跨平台桌面工具，用于 **域名证书监控 + WHOIS 信息查询**，支持 **企业微信 / 钉钉 / 飞书** 通知。

> 本项目由个人独立维护，服务器与域名均为自费。  
> 如果你觉得它对你有帮助，欢迎 ⭐ Star 支持，这对我真的很重要 ❤️

---

## ✨ 功能特性

- ✅ **域名证书监控**
  - 自动检测 HTTPS 证书有效期
  - 支持 HTTP/3 证书检测
  - 证书过期 / 域名到期状态分离

- ✅ **WHOIS 信息查询**
  - 多后缀域名支持（可定制）
  - 精准解析到期时间与注册商

- ✅ **多通道通知**
  - 企业微信
  - 钉钉
  - 飞书

- ✅ **桌面端体验**
  - 开机自启
  - 手动更新机制
  - 轻量、无后台服务依赖

---

## 🖥 支持平台

| 平台 | 状态 |
|----|----|
| Windows 10 | ✅ 已编译测试 |
| macOS | 🚧 计划中 |
| Linux | 🚧 计划中 |

---

## 🚀 快速开始

### 下载使用（推荐）

前往 GitHub Releases 下载 **已编译版本**：

🔗 **[点击前往 GitHub Releases](https://github.com/littletow/bean-domain/releases)**

1. 下载 `bean-domain.zip`
2. 解压并覆盖原目录
3. 启动 `bean-domain.exe`

> ⚠️ 当前采用 **手动更新机制**，以节省服务器带宽。

---

### 源码运行（开发者）
```bash

1. 克隆项目

git clone https://github.com/littletow/bean-domain.git

cd bean-domain

2. 安装依赖

npm install

3. 启动开发模式

wails3 dev

4. 构建生产版本

wails3 build

```

---

## 🔔 通知配置示例

Webhook 配置

本项目支持 企业微信、钉钉、飞书​ 通知。

在配置文件中填写对应平台的 Webhook URL 即可启用。

支持文本消息，适合在微信中直接查看告警内容。

---

## 📜 更新日志

详见：[CHANGELOG](./CHANGELOG.md)

简要摘要：

- **v2.0.0**：项目开源，重构为多文件工程
- **v1.0.8**：优化批量扫描性能
- **v1.0.6**：本地化文档、增强 WHOIS 支持
- **v1.0.3**：支持开机自启

---

## 🛠 技术栈

- **Go + Wails v2**
- **Vue 3 + Naive UI**
- **原生桌面窗口**

---

## 💡 项目初衷

市面上很多证书监控方案：
- 太重（Prometheus / Grafana）
- 太贵（SaaS）
- 太难部署（Docker / Kubernetes）

**SSLChecker 的目标是：**  
> 一个可执行文件，双击即用，适合个人 / 小团队。

---

## ❤️ 支持项目

本项目完全由个人利用业余时间维护，本项目无追踪，无数据收集，可选激励广告支持作者。

如果你觉得它对你有帮助，可以：

- ⭐ **给项目点一个 Star**
- 📣 分享给身边需要的朋友
- 📝 在 GitHub Issues 提出建议
- 💬 关注公众号：**技术源泉**
---

## 📄 协议

Apache License 2.0

---

## 📬 联系作者

- 📫 邮箱：eagle.mon@qq.com
