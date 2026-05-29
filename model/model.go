package model

/**
* 这个文件用来存放共享模型信息。
* 当需要多个模块使用时抽取出来放在这里。
 */

// 更新信息
type UpdateInfo struct {
	LatestVersion string `json:"latestVersion"`
	DownloadUrl   string `json:"downloadUrl"`
	UpdateLog     string `json:"updateLog"`
}

// APP状态
type AppState struct {
	// 设备与系统信息
	DeviceID    string `json:"deviceID"`    // 设备唯一标识 (指纹)
	AppVersion  string `json:"appVersion"`  // 当前软件版本，如 "v1.0.0"
	OS          string `json:"os"`          // 运行平台: windows, darwin, linux
	InstallTime string `json:"installTime"` // 安装时间
	// 运行统计
	// FirstRunTime string `json:"firstRunTime"` // 首次启动时间戳
	// LastRunTime  string `json:"lastRunTime"`  // 最近一次启动时间戳

}

// 存储的域名证书信息
type DomainModel struct {
	Domain      string     `json:"domain"`      // 域名信息
	Port        int        `json:"port"`        // 端口信息，默认443
	UpdatedAt   int64      `json:"updatedAt"`   // 更新时间戳
	NextCheckAt int64      `json:"nextCheckAt"` // 下次检测时间
	Whois       WhoisModel `json:"whois"`       // 域名状态
	SSL         SSLModel   `json:"ssl"`         // 证书状态
}

type WhoisModel struct {
	Expiry       int64  `json:"expiry"`       // 过期时间
	RegisteredAt int64  `json:"registeredAt"` // 注册时间
	LastCheckAt  int64  `json:"lastCheckAt"`  // 上次扫描时间
	Status       string `json:"status"`       // active, expired, error
	LastError    string `json:"lastError"`
}

type SSLModel struct {
	Expiry      int64  `json:"expiry"`      // 证书过期时间
	LastCheckAt int64  `json:"lastCheckAt"` // 上次扫描时间
	Status      string `json:"status"`      // valid, warning, expired, error
	LastError   string `json:"lastError"`
	Issuer      string `json:"issuer"` // 建议增加：签发者信息，方便排查
}

// 调度配置
type ScheduleConfig struct {
	// 最近一次通知时间
	LastNotifyDate string `json:"lastNotifyDate"` // 记录上次发送日期，如 "2026-01-24"
	// 扫描控制
	LastFullScanAt int64 `json:"lastFullScanAt"` // 上次全量扫描时间戳
	// 通知设置
	NotifyTime      string `json:"notifyTime"`      // 每日通知时间点，如 "10:00"
	NotifyThreshold int    `json:"notifyThreshold"` // 提前告警阈值（天），如 30
	EnableNotify    bool   `json:"enableNotify"`    // 是否开启机器人通知

}

// webhook模型
type WebhookModel struct {
	Type      string `json:"type"`       // "dingtalk" 或 "wechat"
	URL       string `json:"url"`        // Webhook 完整地址
	Secret    string `json:"secret"`     // 仅钉钉加签使用
	Enable    bool   `json:"enable"`     // 是否启用
	UpdatedAt int64  `json:"updated_at"` // 更新时间
}

// 存储数据内容
type Store struct {
	Domains        []DomainModel           `json:"domains"`         // 域名列表
	ScheduleConfig ScheduleConfig          `json:"schedule_config"` // 调度配置
	Webhooks       map[string]WebhookModel `json:"webhooks"`        // key 为 "dingtalk" 或 "wechat" 或 "feishu"
	AppState       AppState                `json:"app_state"`       // 应用信息
}

// 以前的数据结构，直接去掉，不进行兼容。
// // 小纸条模型
// type NoteModel struct {
// 	ID    uint   `json:"id"`
// 	Title string `json:"title"`
// 	URL   string `json:"url"`
// }

// // V1版本，包含小纸条
// type StoreV1 struct {
// 	Domains        []DomainModel           `json:"domains"`         // 域名列表
// 	ScheduleConfig ScheduleConfig          `json:"schedule_config"` // 调度配置
// 	Webhooks       map[string]WebhookModel `json:"webhooks"`        // key 为 "dingtalk" 或 "wechat" 或 "feishu"
// 	AppState       AppState                `json:"app_state"`       // 应用信息
// 	Notes          []NoteModel             `json:"notes"`           // 小纸条内容
// }
