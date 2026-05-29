# 🌐 Bean Domain

**BeanDomain** is a cross-platform desktop application built with **Wails v2**, designed for **domain certificate monitoring** and **WHOIS lookup**, with support for **WeCom, DingTalk, and Feishu notifications**.

> This project is independently maintained by an individual developer.  
> Servers and domains are self-funded.  
> If you find it useful, please consider giving it a ⭐ Star — it really means a lot ❤️

---

## ✨ Features

- ✅ **Domain Certificate Monitoring**
  - Automatic HTTPS certificate expiration checks
  - HTTP/3 certificate detection
  - Clear separation of certificate expiry and domain expiry

- ✅ **WHOIS Information Lookup**
  - Multi-TLD support (customizable)
  - Accurate parsing of expiration dates and registrars

- ✅ **Multi-Channel Notifications**
  - WeCom (Enterprise WeChat)
  - DingTalk
  - Feishu

- ✅ **Desktop-First Experience**
  - Start on system boot
  - Manual update mechanism
  - Lightweight, no background services required

---

## 🖥 Supported Platforms

| Platform | Status |
|--------|--------|
| Windows 10 | ✅ Tested |
| macOS | 🚧 Planned |
| Linux | 🚧 Planned |

---

## 🚀 Quick Start

### Download (Recommended)

Download the prebuilt version from GitHub Releases:

🔗 **[Go to GitHub Releases](https://github.com/littletow/bean-domain/releases)**

1. Download `bean-domain.zip`
2. Extract and overwrite the previous installation folder
3. Run `bean-domain.exe`

> ⚠️ Currently uses a manual update mechanism to save server bandwidth.

---

### Run from Source (Developers)
```bash

1. Clone the repository

git clone https://github.com/littletow/bean-domain.git

cd bean-domain

2. Install dependencies

npm install

3. Start development mode

wails3 dev

4. Build production binary

wails3 build


```
---

## 🔔 Notification Configuration Example

Webhook Configuration

This project supports notifications for WeCom, DingTalk, and Feishu.

Simply provide the corresponding webhook URL in the configuration file.

Text-based messages make it easy to view alerts directly on your phone.

---

## 📜 Changelog

See: [CHANGELOG](./CHANGELOG.md)

**Highlights:**
- **v2.0.0**: Project open-sourced, refactored into multi-file architecture
- **v1.0.8**: Improved batch scanning performance
- **v1.0.6**: Localized docs, enhanced WHOIS support
- **v1.0.3**: Added auto-start on boot

---

## 🛠 Tech Stack

- **Go + Wails v3**
- **Vue 3 + Naive UI**
- **Native Desktop Window**

---

## 💡 Why SSLChecker?

Most existing solutions are:
- Too heavy (Prometheus / Grafana)
- Too expensive (SaaS platforms)
- Too complex (Docker / Kubernetes)

**SSLChecker aims to be:**  
> A single executable file, ready to use after double-clicking — perfect for individuals and small teams.

---

## ❤️ Support the Project

This project is fully maintained in my spare time,This project does not track users or collect personal data.  
Optional rewarded ads are used only to support ongoing development.

If you find it helpful, you can:
- ⭐ Star this repository
- 📣 Share it with friends or colleagues
- 📝 Submit suggestions via GitHub Issues

You can also follow my WeChat Official Account **技术源泉**  
for updates, tutorials, and how to support the author ❤️
---

## 📄 License

Apache License 2.0

---

## 📬 Contact
📫 Bug reports / Suggestions / Security issues: 
- 📫 Email: eagle.mon@qq.com


