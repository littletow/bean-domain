package service

import (
	"bean-domain/model"
	"bean-domain/store"
	"bean-domain/util"
	"bean-domain/xnet"
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"math"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

/*
* 这是核心文件，核心设计思想：
* 1，该文件负责管理调度域名检测以及前端展示。
* 2，域名信息及配置，发送给store服务处理，store 提供即时保存和异步保存。
* 3，通过manager 和 wails3 前端进行交互。
* 4，通过xnet 进行网络通信，域名检测。
* 5，通过model 提供共享模型信息。
 */

// 从2.0.0开始，开源项目。

const (
	MaxLimitDomainCount = 100
	AppVersion          = "v2.0.0"
)

// 状态常量化
const (
	StatusPending = "pending"
	StatusValid   = "valid"
	StatusWarning = "warning"
	StatusExpired = "expired"
	StatusError   = "error"
)

// AppManager 统一管理应用和窗口
type AppManager struct {
	App        *application.App
	MainWindow application.Window
}

// 返回前端域名证书信息
type DomainInfo struct {
	ID     int    `json:"id"`     // ID
	Domain string `json:"domain"` // 域名

	// --- 证书状态 (SSL) ---
	SslStatus string `json:"sslStatus"` // valid, warning, expired, error
	SslDate   string `json:"sslDate"`   // 证书到期日期 (2024-12-31)
	SslRemain int    `json:"sslRemain"` // 证书剩余天数
	SslError  string `json:"sslError"`  // 证书扫描的具体报错

	// --- 域名状态 (Whois) ---
	DomainStatus string `json:"domainStatus"` // active, expired, error
	DomainDate   string `json:"domainDate"`   // 域名到期日期 (2025-06-01)
	DomainRemain int    `json:"domainRemain"` // 域名剩余天数
	DomainError  string `json:"domainError"`  // Whois 查询的具体报错

	// --- 综合信息 ---
	LastCheckAt string `json:"lastCheckAt"` // 上次成功更新的时间
}

// 验证结果
type VerifyResult struct {
	Valid          []string `json:"valid"`
	Duplicate      []string `json:"duplicate"`
	Invalid        []string `json:"invalid"`
	CurrentTotal   int      `json:"currentTotal"`   // 当前已存在数量
	RemainingQuota int      `json:"remainingQuota"` // 剩余可用名额
}

// DashboardStats 仪表盘统计数据结构
type DashboardStats struct {
	CurrentTotal int `json:"currentTotal"`
	MaxLimit     int `json:"maxLimit"`

	// --- 细化统计维度 ---
	AlertCount       int `json:"alertCount"`       // 总预警数 (进入阈值或扫描失败)
	SslAlertCount    int `json:"sslAlertCount"`    // 仅证书预警/失败
	DomainAlertCount int `json:"domainAlertCount"` // 仅域名预警/失败
	ExpiredCount     int `json:"expiredCount"`     // 已过期总数 (SSL过期或域名过期)

	NotifyThreshold int             `json:"notifyThreshold"`
	NotifyTime      string          `json:"notifyTime"`
	LastScanTime    string          `json:"lastScanTime"`
	NextScanTime    string          `json:"nextScanTime"`
	WebhookStatus   map[string]bool `json:"webhookStatus"`
	UpdatedFields   map[string]bool `json:"updatedFields"`
}

// 快照
type StatsSnapshot struct {
	CurrentTotal     int             `json:"currentTotal"`
	AlertCount       int             `json:"alertCount"`
	SslAlertCount    int             `json:"sslAlertCount"`
	DomainAlertCount int             `json:"domainAlertCount"`
	ExpiredCount     int             `json:"expiredCount"`
	NotifyThreshold  int             `json:"notifyThreshold"`
	NotifyTime       string          `json:"notifyTime"`
	LastScanTime     string          `json:"lastScanTime"`
	WebhookStatus    map[string]bool `json:"webhookStatus"`
}

// 域名服务
type DomainService struct {
	mu            sync.RWMutex
	ctx           context.Context
	store         *store.DataStore
	manager       *AppManager
	stopScheduler chan struct{} // 用于停止旧的调度协程
	lastStats     *StatsSnapshot
	domainMap     map[string]*model.DomainModel
	scanChan      chan ScanTask // 扫描任务队列
	activeTasks   atomic.Int32
}

type ScanTask struct {
	Target     *model.DomainModel // 内存中域名对象的指针
	Index      int
	Total      int
	ForceSSL   bool
	ForceWhois bool
}

type ReportStats struct {
	Total       int      // 总数量
	Safe        int      // 安全数量
	SslAlert    int      // 证书预警/错误数
	DomainAlert int      // 域名预警/错误数
	Expired     int      // 已过期总数
	AlertList   []string // 建议格式化为 "example.com (证书剩3天)"
}

type ScanStatus struct {
	IsScanning    bool   `json:"is_scanning"`
	CurrentDomain string `json:"current_domain"`
	CurrentIndex  int    `json:"current_index"`
	TotalCount    int    `json:"total_count"`
}

type MpcodeStatus struct {
	Status  string `json:"status"`
	Message string `json:"msg"`
}

func NewDomainService(ds *store.DataStore, manager *AppManager) *DomainService {
	s := &DomainService{
		store:         ds,
		manager:       manager,
		stopScheduler: make(chan struct{}),
		scanChan:      make(chan ScanTask, 100), // 缓冲队列
	}
	// 启动一个后台消费者，慢慢处理
	go s.startBackgroundScanner()
	return s
}

func (s *DomainService) startBackgroundScanner() {
	fmt.Println("启动后台扫描任务")
	// 这个协程在后台永远运行，一个接一个处理

	for task := range s.scanChan {
		fmt.Println("开始执行扫描任务", task.Target.Domain)
		s.setScanStatus(true, task.Target.Domain, task.Index, task.Total)

		s.refreshDomainData(task)
		s.activeTasks.Add(-1)
		if s.activeTasks.Load() <= 0 {
			s.setScanStatus(false, "", 0, 0)
		}
	}
}

// 内部统一生成 key 的规则，避免到处拼接字符串
func (s *DomainService) getMapKey(domain string, port int) string {
	// 如果端口是 0，默认设为 443 保持一致性
	p := port
	if p == 0 {
		p = 443
	}
	return fmt.Sprintf("%s:%d", domain, p)
}

// RebuildIndex 当 Domains 切片发生增删或扩容时，必须调用此函数
func (s *DomainService) RebuildIndex() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 重新初始化 Map
	s.domainMap = make(map[string]*model.DomainModel)

	// 2. 遍历 store 里的原始切片
	for i := range s.store.Data.Domains {
		// 关键：取的是 Data.Domains 里的真实元素地址
		dm := &s.store.Data.Domains[i]
		// 使用 域名+端口 作为唯一标识
		key := s.getMapKey(dm.Domain, dm.Port)
		s.domainMap[key] = dm
	}
}

// 初始化系统状态
func (s *DomainService) InitAppState(devID string) {
	// 不用加锁，因为只在主线程执行一次
	// s.mu.Lock()

	currDate := time.Now().Format(time.DateTime)
	s.store.Data.AppState.DeviceID = devID
	s.store.Data.AppState.InstallTime = currDate
	// 2. 填充系统信息
	s.store.Data.AppState.AppVersion = AppVersion
	s.store.Data.AppState.OS = runtime.GOOS

	// s.mu.Unlock() // 尽早解锁

	// 3. 标记脏数据并立即执行一次存盘 (确保启动信息第一时间记入磁盘)
	s.store.MarkDirty()
	_ = s.store.SaveNow()

}

// 是否首次运行
func (s *DomainService) IsFirstRun() bool {
	ok := s.store.ExistsKeyFile()
	return !ok
}

func (s *DomainService) GetMpCode() string {
	mpcode := xnet.GetMpCode()
	// fmt.Println("小程序码，", mpcode)
	if mpcode != "" {
		go s.wsSetPassword()
	}

	return mpcode
}

func (s *DomainService) wsSetPassword() {
	result, err := xnet.WsHandleSetPassword()
	if err != nil {
		s.notifyMpcodeSetPassword("fail", err.Error())
		return
	}

	resultArr := strings.Split(result, ",")
	devID := resultArr[0]
	password := resultArr[1]

	err = s.store.Init(password)
	if err != nil {
		log.Println("客户端初始化失败，", err)
		s.notifyMpcodeSetPassword("fail", err.Error())
		return
	}

	err = s.store.Load()
	if err != nil {
		log.Println("加载数据文件时错误，", err)
		s.notifyMpcodeSetPassword("fail", err.Error())
		return
	}

	// 初始化 App 状态
	s.InitAppState(devID)

	// 重建内存索引
	s.RebuildIndex()

	s.notifyMpcodeSetPassword("ok", "设置成功")

}

// 客户端初始化，创建加密数据文件
func (s *DomainService) SetPassword(password string) error {
	err := s.store.Init(password)
	if err != nil {
		log.Println("客户端初始化失败，", err)
	}

	err = s.store.Load()
	if err != nil {
		log.Println("加载数据文件时错误，", err)
		return err
	}
	devID := util.GenDeviceID()
	// 初始化 App 状态
	s.InitAppState(devID)

	// 重建内存索引
	s.RebuildIndex()

	return nil
}

// 随服务启动
func (s *DomainService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.ctx = ctx

	// ✅ 1. 判断是否是首次启动
	if s.IsFirstRun() {
		// 不初始化任何东西
		// 只告诉前端：需要初始化
		return nil
	}
	// 加载数据内容
	err := s.store.Load()
	if err != nil {
		log.Println("加载数据文件时错误，", err)
		return err
	}

	// 1. 建立内存索引（核心：直接从 store.Data.Domains 建立指针映射）
	s.RebuildIndex()

	// 从数据库读取配置
	cfg, err := s.GetScheduleConfig()
	if err == nil {
		// 启动全量扫描任务
		s.startFullScanManager(cfg.LastFullScanAt)
		// 启动初始调度器
		s.restartNotificationScheduler(cfg)
	}

	return nil
}

func newDomain(name string, port int) model.DomainModel {
	now := time.Now().Unix()
	return model.DomainModel{
		Domain:      name,
		Port:        port,
		UpdatedAt:   now,
		NextCheckAt: now, // 立即进入队列等待首次扫描
		// 域名初始状态
		Whois: model.WhoisModel{
			Status:       StatusPending, // 等待扫描
			RegisteredAt: 0,
			Expiry:       0,
			LastCheckAt:  0,
			LastError:    "",
		},

		// 证书初始状态
		SSL: model.SSLModel{
			Status:      StatusPending, // 等待扫描
			Expiry:      0,
			LastCheckAt: 0,
			LastError:   "",
			Issuer:      "",
		},
	}
}

func (s *DomainService) SaveImportedDomains(domains []string) error {
	if s.store == nil {
		return fmt.Errorf("存储系统未就绪")
	}

	s.mu.Lock()
	// 1. 检查配额
	currentCount := len(s.store.Data.Domains)
	available := MaxLimitDomainCount - currentCount
	if available <= 0 {
		s.mu.Unlock() // 记得手动解锁
		return fmt.Errorf("配额已满 (%d/%d)", currentCount, MaxLimitDomainCount)
	}

	// 2. 预处理：去重 Map
	existingMap := make(map[string]bool)
	for _, v := range s.store.Data.Domains {
		existingMap[strings.ToLower(v.Domain)] = true
	}

	// 3. 收集真正需要新增的域名字符串
	var newNames []string
	for _, name := range domains {
		if len(newNames) >= available {
			break
		}
		lname := strings.ToLower(strings.TrimSpace(name))
		if lname == "" || existingMap[lname] {
			continue
		}
		newNames = append(newNames, lname)
	}

	// 4. 执行批量新增
	if len(newNames) > 0 {
		for _, name := range newNames {
			domain, port := parseDomainPort(name) // 建议抽离个小工具函数
			s.store.Data.Domains = append(s.store.Data.Domains, newDomain(domain, port))
		}

		// --- 关键点：操作完 append，立刻重建索引 ---
		// 这步执行完，s.domainMap 里的指针就全部更新为指向 Data.Domains 里的新地址了
		s.mu.Unlock() // 先解锁，RebuildIndex 内部会自己加锁
		s.RebuildIndex()

		// --- 关键点：从更新后的 Map 中获取“真实指针”发给扫描器 ---
		var tasks []*model.DomainModel
		s.mu.RLock()
		for _, name := range newNames {
			domain, port := parseDomainPort(name)
			key := s.getMapKey(domain, port)
			if ptr, ok := s.domainMap[key]; ok {
				tasks = append(tasks, ptr)
			}
		}
		s.mu.RUnlock()

		// 5. 立即持久化并触发扫描
		s.store.SaveNow()
		go s.batchRefreshDomains(tasks)
	} else {
		s.mu.Unlock()
	}

	s.notifyConfigRefresh("domain")
	s.notifyDomainRefresh("domain")
	return nil
}

// 辅助工具：提取端口逻辑
func parseDomainPort(input string) (string, int) {
	domain := input
	port := 443
	if strings.Contains(input, ":") {
		parts := strings.Split(input, ":")
		domain = parts[0]
		p, _ := strconv.Atoi(parts[1])
		if p > 0 {
			port = p
		}
	}
	return domain, port
}

// BatchDeleteDomainsByNames 根据域名名称批量删除
func (s *DomainService) BatchDeleteDomainsByNames(domainNames []string) error {
	s.mu.Lock()

	// 1. 将待删除域名组装成 Map
	deleteMap := make(map[string]struct{})
	for _, name := range domainNames {
		domain, port := parseDomainPort(name)
		key := s.getMapKey(domain, port)
		deleteMap[key] = struct{}{}
	}

	// 2. 过滤掉匹配的域名 (原地过滤，减少内存分配)
	originalDomains := s.store.Data.Domains
	newDomains := make([]model.DomainModel, 0, len(originalDomains))

	for _, d := range originalDomains {
		key := s.getMapKey(d.Domain, d.Port)
		if _, exists := deleteMap[key]; !exists {
			newDomains = append(newDomains, d)
		}
	}
	// 3. 更新原始数据
	s.store.Data.Domains = newDomains
	s.mu.Unlock() // 提前解锁

	// 4. 【关键】重建索引：因为切片内容变了，必须清理旧指针并建立新映射
	s.RebuildIndex()

	// 5. 【关键】立即存盘：手动删除后建议立即同步到磁盘
	s.store.MarkDirty()
	if err := s.store.SaveNow(); err != nil {
		fmt.Printf("❌ 删除持久化失败: %v\n", err)
	}

	// 6. 通知 UI 更新
	s.notifyConfigRefresh("domain")
	s.notifyDomainRefresh("domain") // 通知域名列表刷新

	return nil
}

// 批量扫描
func (s *DomainService) BatchRefreshDomains(domains []string) error {
	s.mu.RLock() // 只是读取 Map，用 RLock 性能更高
	// 1. 预分配容量，避免循环中频繁扩容
	addedDomains := make([]*model.DomainModel, 0, len(domains))

	for _, name := range domains {
		// 2. 获取内存中的原始指针
		// 关键：这里需要解析出域名和端口，才能拼出 Key
		domain, port := parseDomainPort(name)
		key := s.getMapKey(domain, port)

		if modelPtr, ok := s.domainMap[key]; ok {
			addedDomains = append(addedDomains, modelPtr)
		}
	}
	s.mu.RUnlock()
	// 3. 防御性检查：如果没有有效域名，直接退出
	if len(addedDomains) == 0 {
		return nil
	}
	// 4. 提交给批量扫描逻辑
	return s.batchRefreshDomains(addedDomains)
}

// 批量刷新域名证书
func (s *DomainService) batchRefreshDomains(domains []*model.DomainModel) error {
	total := len(domains)
	for i, dm := range domains {
		// 构造任务：直接把内存对象的指针 dm 塞进去
		task := ScanTask{
			Target:     dm, // 核心：传递指针
			Index:      i + 1,
			Total:      total,
			ForceSSL:   true,
			ForceWhois: false,
		}

		// 利用指针直接判断是否需要更新 Whois
		if s.isWhoisStale(dm) {
			task.ForceWhois = true
		}

		select {
		case s.scanChan <- task:
			s.activeTasks.Add(1)
			fmt.Printf("✅ [%d/%d] %s 已加入队列\n", i+1, total, dm.Domain)
		default:
			// 如果 100 个缓冲区的 scanChan 满了，说明消费者处理太慢
			fmt.Printf("⚠️ 扫描队列已满，跳过: %s\n", dm.Domain)
		}
	}

	return nil
}

// GetTotalCount 获取当前已保存的域名总数
func (s *DomainService) getTotalCount() int {
	s.mu.RLock() // 使用读锁，提高并发效率
	defer s.mu.RUnlock()

	return len(s.store.Data.Domains)
}

func (s *DomainService) runSSLCheck(domain string, port int) (int64, error) {
	time.Sleep(500 * time.Millisecond)
	ts, err := xnet.SSLCheck(domain, port)
	return ts, err
}

func (s *DomainService) runWhoisCheck(domain string) (int64, error) {
	// 1. 引入随机延迟 (1000ms - 3000ms)
	// 防止被 Whois Server 识别为固定频率攻击
	randDelay := rand.Intn(2000) + 3000
	time.Sleep(time.Duration(randDelay) * time.Millisecond)
	ts, err := xnet.WhoisCheck(domain)

	return ts, err
}

// GetDomainList 获取所有监控域名列表
func (s *DomainService) GetDomainList() ([]DomainInfo, error) {
	s.mu.RLock() // 只需要读锁
	defer s.mu.RUnlock()

	domains := s.store.Data.Domains
	list := make([]DomainInfo, 0, len(domains)) // 预分配内存
	now := time.Now().Unix()

	for i, m := range domains {
		// 1. 处理显示用的域名和端口
		displayDomain := m.Domain
		if m.Port != 0 && m.Port != 443 {
			displayDomain = fmt.Sprintf("%s:%d", m.Domain, m.Port)
		}

		// 2. 构造返回对象
		resp := DomainInfo{
			ID:     i + 1,
			Domain: displayDomain,

			// 证书维度
			SslStatus: m.SSL.Status,
			SslDate:   util.FormatTimestamp(m.SSL.Expiry),
			SslError:  m.SSL.LastError,
			SslRemain: calculateRemainDays(m.SSL.Expiry, m.SSL.Status, now),

			// 域名维度
			DomainStatus: m.Whois.Status,
			DomainDate:   util.FormatTimestamp(m.Whois.Expiry),
			DomainError:  m.Whois.LastError,
			DomainRemain: calculateRemainDays(m.Whois.Expiry, m.Whois.Status, now),

			LastCheckAt: util.FormatTimestamp(m.UpdatedAt),
		}

		list = append(list, resp)
	}

	return list, nil
}

// 辅助函数：统一计算剩余天数
func calculateRemainDays(expiry int64, status string, now int64) int {
	if expiry <= 0 || status == StatusError || status == StatusPending {
		return -999
	}
	// 计算天数（向上取整或直接相减，取决于业务需求，这里保持你原有的逻辑）
	return int((expiry - now) / 86400)
}

/*
* ### 📊 域名监控日报 (2026-01-22)
---
✅ **运行正常**: 120 个
⚠️ **即将到期**: 5 个
❌ **已经过期**: 3 个

**待处理告警域名列表:**
1. \`test-expired.com\`
2. \`demo-old.net\`
3. \`api-v1.io\`

> 豆子域名管家提醒您，请及时处理告警域名。
*/
func (s *DomainService) sendDailyReport(cfg model.ScheduleConfig) error {
	log.Println("发送报表参数", cfg.EnableNotify, cfg.NotifyTime, cfg.LastNotifyDate, cfg.NotifyThreshold, cfg.LastFullScanAt)
	// 没有域名不发送
	if len(s.store.Data.Domains) == 0 {
		return nil
	}

	if s.store.Data.ScheduleConfig.LastFullScanAt == 0 {
		return nil
	}

	now := time.Now()
	todayStr := now.Format(time.DateTime)
	content := s.generateTextDailyReport()
	s.pushToWebhooks(content)
	return s.updateLastNotifyDate(todayStr)
}

func (s *DomainService) pushToWebhooks(content string) {
	// 发送钉钉
	if dd, err := s.getWebhook("dingtalk"); err == nil && dd.URL != "" {
		s.sendDingTalk(dd.URL, dd.Secret, content)
	}

	// 发送企微
	if wc, err := s.getWebhook("wechat"); err == nil && wc.URL != "" {
		s.sendWeChat(wc.URL, content)
	}

	// 发送飞书
	if fs, err := s.getWebhook("feishu"); err == nil && fs.URL != "" {
		s.sendFeiShu(fs.URL, fs.Secret, content)
	}
}

func (s *DomainService) getWebhook(msgType string) (*model.WebhookModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	model, ok := s.store.Data.Webhooks[msgType]
	if ok {
		return &model, nil
	}
	return nil, fmt.Errorf("类型 %s 未配置", msgType)
}

func (s *DomainService) sendWeChat(url string, content string) {
	xnet.WechatNotify(url, content)
}

func (s *DomainService) sendDingTalk(webhookURL string, secret string, content string) {
	xnet.DingtalkNotify(webhookURL, content, secret)
}

func (s *DomainService) sendFeiShu(webhookURL string, secret string, content string) {
	xnet.FeishuNotify(webhookURL, content, secret)
}

func shouldRetryNotify(lastNotifyDate string, notifyTimeStr string) bool {
	if lastNotifyDate == "" {
		fmt.Println("最近通知日期未设置")
		return false
	}

	t1, err := time.Parse(time.DateTime, lastNotifyDate)
	if err != nil {
		fmt.Println("最近通知日期格式错误:", err)
		return false
	}

	// 1. 如果今天已经通知过了，直接跳过
	now := time.Now()
	if t1.Year() == now.Year() &&
		t1.Month() == now.Month() &&
		t1.Day() == now.Day() {
		return false
	}

	// 2. 解析通知时间 (格式 "15:04")
	t2, err := time.Parse("15:04", notifyTimeStr)
	if err != nil {
		fmt.Println("通知时间格式错误:", err)
		return false
	}

	// 3. 构造今天的目标通知时间点
	todayNotifyTime := time.Date(
		now.Year(), now.Month(), now.Day(),
		t2.Hour(), t2.Minute(), 0, 0, now.Location(),
	)

	// 4. 如果当前时间已经过了今天的通知时间，且今天没通知过，则补发
	return now.After(todayNotifyTime)
}

func (s *DomainService) restartNotificationScheduler(cfg model.ScheduleConfig) {
	// 1. 如果之前有运行中的调度器，先发送停止信号
	if s.stopScheduler != nil {
		close(s.stopScheduler)
	}

	// 2. 创建一个新的停止通道
	s.stopScheduler = make(chan struct{})

	// 3. 开启新的后台检查协程
	go func(stop chan struct{}, currentCfg model.ScheduleConfig) {
		// 检查最近一次通知时间，如果还未到通知时间，则不通知，如果已经过期了，今天还没有通知则补充一次
		ok := shouldRetryNotify(currentCfg.LastNotifyDate, currentCfg.NotifyTime)
		if ok {
			s.sendDailyReport(currentCfg)
		}
		// 每一分钟检查一次时间
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 重新从数据库读取最新配置（防止运行期间配置又改了）
				latestCfg, err := s.GetScheduleConfig()
				if err != nil || !latestCfg.EnableNotify {
					continue
				}

				// 检查当前时间是否匹配，例如 "10:00"
				if time.Now().Format("15:04") == latestCfg.NotifyTime {
					s.sendDailyReport(latestCfg)
				}
			case <-stop:
				// 接收到停止信号，退出当前协程
				return
			case <-s.ctx.Done():
				// Wails 程序关闭，退出协程
				return
			}
		}
	}(s.stopScheduler, cfg)
}

func (s *DomainService) notifyDomainRefresh(domain string) {
	s.manager.App.Event.Emit("domain-refreshed", domain)
}

func (s *DomainService) notifyConfigRefresh(config string) {
	s.manager.App.Event.Emit("config-updated", config)
}

func (s *DomainService) notifyMpcodeSetPassword(status string, msg string) {
	statusMsg := MpcodeStatus{
		Status:  status,
		Message: msg,
	}
	s.manager.App.Event.Emit("mpcode-status", statusMsg)
}

func (s *DomainService) setScanStatus(active bool, domain string, index int, total int) {
	status := ScanStatus{
		IsScanning:    active,
		CurrentDomain: domain,
		CurrentIndex:  index,
		TotalCount:    total,
	}
	s.manager.App.Event.Emit("scan-status", status)
}

func (s *DomainService) updateLastNotifyDate(dateStr string) error {
	s.mu.Lock()
	// 仅更新配置中的日期字段
	s.store.Data.ScheduleConfig.LastNotifyDate = dateStr
	s.mu.Unlock() // 尽早解锁

	// 1. 标记脏数据（触发后台 5s 自动保存）
	s.store.MarkDirty()

	// 2. 核心状态：建议立即强制存盘，防止宕机导致重复通知
	if err := s.store.SaveNow(); err != nil {
		fmt.Printf("⚠️ 警告：通知日期持久化失败: %v\n", err)
	}

	// 3. 通知前端 UI 更新配置显示
	s.notifyConfigRefresh("schedule")

	return nil
}

// 辅助方法：安全更新时间戳
func (s *DomainService) updateLastFullScanAt(ts int64) {
	s.mu.Lock()
	s.store.Data.ScheduleConfig.LastFullScanAt = ts
	s.mu.Unlock()       // 尽早解锁
	s.store.MarkDirty() // 标记脏数据
	s.notifyConfigRefresh("schedule")
}

// 执行扫描服务
func (s *DomainService) startFullScanManager(lastScanAt int64) {
	// 使用 context 确保协程可以随 Service 销毁
	go func() {
		// 1. 【新增】冷启动避灾延迟：给客户端 5 秒钟处理其他初始化任务
		time.Sleep(5 * time.Second)
		for {
			now := time.Now()
			lastScanTime := time.Unix(lastScanAt, 0)

			// 判断是否是“新的一天”，按日期比较
			// 如果上次扫描日期不是今天，说明该重新扫描了
			isNewDay := now.Format("2006-01-02") != lastScanTime.Format("2006-01-02")

			// 计算24小时的物理等待时间
			nextScanAt := lastScanAt + 86400
			// 计算距离下次扫描的等待时长
			waitTime := time.Until(time.Unix(nextScanAt, 0))

			// 核心逻辑判断
			if isNewDay || waitTime <= 0 {
				// 满足：[已经是第二天了] 或者 [物理时间已过 24 小时]
				fmt.Println("[调度器] 检测到新的一天或逾期，准备执行扫描...")
				waitTime = 0
			}

			if waitTime > 0 {
				fmt.Printf("[调度器] 今日已扫，下次扫描时间: %v, 等待时长: %v\n",
					time.Unix(nextScanAt, 0).Format("2006-01-02 15:04:05"), waitTime.Round(time.Second))

				// 创建定时器
				timer := time.NewTimer(waitTime)

				select {
				case <-timer.C:
					// 时间到了，继续往下执行扫描
					fmt.Println("[调度器] 接收到扫描信号，准备开始扫描...")
				case <-s.ctx.Done():
					// 监听到客户端关闭信号，优雅退出
					timer.Stop()
					fmt.Println("[调度器] 接收到退出信号，正在关闭扫描协程...")
					return
				}
			}

			// --- 更新并持久化时间 ---
			lastScanAt = time.Now().Unix()
			// --- 执行扫描任务 ---
			s.runDailyScan()
			// 确保扫描完成后立刻存入你的持久化存储
			// 3. 更新全局扫描记录时间，防止重复触发
			s.updateLastFullScanAt(lastScanAt)
			// 循环自动进入下一轮，基于新的 lastScanAt 计算下个 24 小时
		}
	}()
}

// 辅助判断：Whois 是否过期（如 7 天更新一次）
func (s *DomainService) isWhoisStale(dm *model.DomainModel) bool {
	now := time.Now().Unix()

	// 1. 基础间距计算
	gap := now - dm.Whois.LastCheckAt

	// 2. 如果从未成功检查过，必须检查
	if dm.Whois.LastCheckAt == 0 {
		return true
	}

	// 3. 计算剩余天数（取最大值 0，防止过期太久变成负数干扰判断）
	remainingSec := dm.Whois.Expiry - now
	if remainingSec < 0 {
		remainingSec = 0
	}
	daysLeft := remainingSec / 86400

	// 4. 组合策略：
	// - 距离过期不足 30 天时，缩短检查周期至每天一次 (gap > 86400)
	// - 正常情况下，每 7 天全量更新一次 (gap > 7*86400)
	if (daysLeft < 30 && gap > 86400) || gap > 7*86400 {
		return true
	}

	return false
}

func (s *DomainService) runDailyScan() {
	s.mu.RLock()
	// 获取当前所有域名的指针快照
	var targets []*model.DomainModel
	for _, dm := range s.domainMap {
		targets = append(targets, dm)
	}
	s.mu.RUnlock()

	total := len(targets)
	for i, dm := range targets {
		// 生成任务，直接携带 Target 指针
		task := ScanTask{
			Target:     dm,
			Index:      i + 1,
			Total:      total,
			ForceSSL:   true,
			ForceWhois: s.isWhoisStale(dm),
		}

		s.activeTasks.Add(1)
		s.scanChan <- task // 塞入队列，由后台消费者慢慢扫
	}
}

func (s *DomainService) refreshDomainData(task ScanTask) {
	// 直接通过指针引用内存中的对象
	dm := task.Target
	fmt.Printf("🔍 [%d/%d] 正在处理: %s:%d\n", task.Index, task.Total, dm.Domain, dm.Port)

	var (
		newSSLExp   int64
		newWhoisExp int64
		sslErr      error
		whoisErr    error
	)

	// --- 1. 执行 SSL 检测 ---
	if task.ForceSSL {
		fmt.Println("开始执行SSL扫描任务")
		newSSLExp, sslErr = s.runSSLCheck(dm.Domain, dm.Port)
		status, errMsg := s.calcDimensionStatus(newSSLExp, sslErr)

		s.mu.Lock() // 仅在赋值时加锁，不阻塞网络请求
		dm.SSL.Status = status
		dm.SSL.LastError = errMsg
		if sslErr == nil {
			dm.SSL.Expiry = newSSLExp
			dm.SSL.LastCheckAt = time.Now().Unix()
		}
		s.mu.Unlock()
	}

	// --- 2. 执行 Whois 检测 ---
	if task.ForceWhois {
		fmt.Println("开始执行Whois扫描任务")
		newWhoisExp, whoisErr = s.runWhoisCheck(dm.Domain)
		status, errMsg := s.calcDimensionStatus(newWhoisExp, whoisErr)

		s.mu.Lock()
		dm.Whois.Status = status
		dm.Whois.LastError = errMsg
		if whoisErr == nil {
			dm.Whois.Expiry = newWhoisExp
			dm.Whois.LastCheckAt = time.Now().Unix()
		}
		s.mu.Unlock()
	}

	// --- 3. 后续处理 ---
	// 只要扫描过，就标记为脏数据，触发 store.go 的延迟写入
	s.store.MarkDirty()

	if sslErr != nil || whoisErr != nil {
		// TODO 上报错误，计划使用sentry-go
	}

	// 通知 UI 刷新 (利用你刚才注入的 manager)
	s.notifyDomainRefresh(dm.Domain)
	s.notifyConfigRefresh("domain")
}

// 统一的状态计算逻辑
func (s *DomainService) calcDimensionStatus(expiry int64, scanErr error) (string, string) {
	// 1. 如果扫描过程报错了，状态直接定死为 error
	if scanErr != nil {
		return "error", scanErr.Error()
	}

	// 2. 如果没报错但没有获取到日期（比如接口返回空）
	if expiry <= 0 {
		return "pending", "等待获取数据"
	}

	// 3. 正常获取到日期，开始计算时间维度
	now := time.Now().Unix()

	// 检查是否已过期 (最高优先级)
	if expiry < now {
		return "expired", ""
	}

	// 检查是否进入预警期
	threshold := int64(s.store.Data.ScheduleConfig.NotifyThreshold)
	if threshold <= 0 {
		threshold = 7
	}
	thresholdSec := threshold * 24 * 3600

	if (expiry - now) < thresholdSec {
		return "warning", ""
	}

	// 4. 全部正常
	return "valid", "" // 对应前端绿色的安全状态
}

// 获取本地版本号
func (s *DomainService) GetAppVersion() string {
	return AppVersion
}

// 获取远端版本号
func (s *DomainService) GetRemoteVersion() (model.UpdateInfo, error) {
	return xnet.GetRemoteVersion()
}

func (s *DomainService) generateTextDailyReport() string {
	s.mu.RLock() // 仅读取建议用 RLock
	defer s.mu.RUnlock()

	now := time.Now().Unix()
	threshold := int64(s.store.Data.ScheduleConfig.NotifyThreshold)
	const secondsPerDay = 86400

	stats := ReportStats{
		Total: len(s.store.Data.Domains),
	}

	var alertList []string
	safeCount := 0

	for _, d := range s.store.Data.Domains {
		isIssue := false
		var reasons []string

		// --- 1. 检查证书维度 ---
		if d.SSL.Status == "error" {
			reasons = append(reasons, "证书扫描失败")
			isIssue = true
			stats.SslAlert++
		} else {
			sslDays := int64(math.Floor(float64(d.SSL.Expiry-now) / secondsPerDay))
			if sslDays < 0 {
				reasons = append(reasons, "证书已过期")
				isIssue = true
				stats.Expired++
			} else if sslDays <= threshold {
				reasons = append(reasons, fmt.Sprintf("证书剩%d天", sslDays))
				isIssue = true
				stats.SslAlert++
			}
		}

		// --- 2. 检查域名维度 ---
		if d.Whois.Status == "error" {
			reasons = append(reasons, "域名信息获取失败")
			isIssue = true
			stats.DomainAlert++
		} else {
			domainDays := int64(math.Floor(float64(d.Whois.Expiry-now) / secondsPerDay))
			if domainDays < 0 {
				reasons = append(reasons, "域名已过期")
				isIssue = true
				stats.Expired++ // 这里可以根据需求决定是否重复计入 Expired
			} else if domainDays <= threshold {
				reasons = append(reasons, fmt.Sprintf("域名剩%d天", domainDays))
				isIssue = true
				stats.DomainAlert++
			}
		}

		// --- 3. 汇总告警信息 ---
		if isIssue {
			// 格式：`example.com` (证书剩2天, 域名信息获取失败)
			alertList = append(alertList, fmt.Sprintf("`%s` (%s)", d.Domain, strings.Join(reasons, ", ")))
		} else {
			safeCount++
		}
	}

	stats.Safe = safeCount // 确保 ReportStats 结构体中有这个字段

	// 4. 截断逻辑保持不变
	const maxDisplay = 20
	if len(alertList) > maxDisplay {
		stats.AlertList = append(alertList[:maxDisplay], fmt.Sprintf("...及其他 %d 项告警", len(alertList)-maxDisplay))
	} else {
		stats.AlertList = alertList
	}

	return s.formatTextReport(stats)
}

func (s *DomainService) formatTextReport(stats ReportStats) string {
	now := time.Now()
	dateStr := now.Format("2006-01-02")

	// 1. 标题与状态分级
	var report strings.Builder
	fmt.Fprintf(&report, "📊 域名资产监控简报 (%s)\n", dateStr)
	report.WriteString("━━━━━━━━━━━━\n")

	// 2. 核心概览 (使用分栏感)
	fmt.Fprintf(&report, "📝 监控总数：%d 个\n", stats.Total)

	// 计算正常数 (总数 - 证书风险 - 域名风险 + 重复项)
	// 或者直接传入 Safe 数。这里假设逻辑已在调用前算好
	safeCount := max(stats.Total-(stats.SslAlert+stats.DomainAlert), 0)

	fmt.Fprintf(&report, "✅ 运行正常：%d 个\n", safeCount)

	// 3. 风险预警 (分类展示，更有针对性)
	hasRisk := stats.SslAlert > 0 || stats.DomainAlert > 0 || stats.Expired > 0

	if hasRisk {
		report.WriteString("\n【风险预警】\n")
		if stats.SslAlert > 0 {
			fmt.Fprintf(&report, "🔐 证书风险：%d 个 (到期/报错)\n", stats.SslAlert)
		}
		if stats.DomainAlert > 0 {
			fmt.Fprintf(&report, "🌐 域名风险：%d 个 (到期/报错)\n", stats.DomainAlert)
		}
		if stats.Expired > 0 {
			fmt.Fprintf(&report, "❌ 已经过期：%d 个\n", stats.Expired)
		}
	}

	// 4. 详细清单 (增加类型标注)
	if len(stats.AlertList) > 0 {
		report.WriteString("\n🚨 待处理详情：\n")
		for i, item := range stats.AlertList {
			// 假设 AlertList 传入时已处理为 "domain.com [SSL/域名]"
			fmt.Fprintf(&report, "%d. %s\n", i+1, item)
		}
	} else {
		report.WriteString("\n✨ 当前所有资产状态良好，无需处理。\n")
	}

	// 5. 底部装饰
	report.WriteString("━━━━━━━━━━━━\n")
	fmt.Fprintf(&report, "💡 豆子域名管家：请及时处理风险项\n⏰ 更新于：%s", now.Format("15:04"))

	return report.String()
}

func (a *DomainService) CheckAutoStartStatus() bool {
	return IsAutoStartEnabled()
}

// SaveWebhook 保存配置到 Store 并持久化
func (s *DomainService) SaveWebhook(platform string, m model.WebhookModel) error {
	s.mu.Lock()

	// 1. 初始化 Map（防御性编程）
	if s.store.Data.Webhooks == nil {
		s.store.Data.Webhooks = make(map[string]model.WebhookModel)
	}

	// 2. 更新元数据
	m.UpdatedAt = time.Now().Unix()
	m.Type = platform
	// 注意：这里建议由前端决定 Enable，或者仅在新建时设为 true
	// 如果用户只是想改一下 URL 但保持禁用，这里强制设为 true 会违背用户意愿
	m.Enable = true

	// 3. 存入内存
	s.store.Data.Webhooks[platform] = m
	s.mu.Unlock() // 提前解锁

	// 4. 【优化】标记并立即存盘
	// Webhook 配置属于关键资产，建议和导入域名一样，手动操作后立即 SaveNow
	s.store.MarkDirty()
	if err := s.store.SaveNow(); err != nil {
		fmt.Printf("❌ Webhook 持久化失败: %v\n", err)
		return err
	}

	// 5. 通知 UI 刷新
	s.notifyConfigRefresh("webhook")

	return nil
}

// sendToWebhook 核心发送逻辑
func (s *DomainService) sendToWebhook(config model.WebhookModel, content string) error {

	finalURL := config.URL
	// 2. 构造消息体 (企微和钉钉均支持 text 类型)
	payload := make(map[string]interface{})

	if config.Type == "dingtalk" || config.Type == "wechat" {
		payload["msgtype"] = "text"
		payload["text"] = map[string]string{
			"content": content,
		}
	}

	if config.Type == "feishu" {
		payload["msg_type"] = "text"
		payload["content"] = map[string]string{
			"text": content,
		}
	}

	// 1. 如果是钉钉且配置了 Secret，计算签名
	if config.Type == "dingtalk" && config.Secret != "" {
		timestamp := time.Now().UnixNano() / 1e6
		stringToSign := fmt.Sprintf("%d\n%s", timestamp, config.Secret)

		h := hmac.New(sha256.New, []byte(config.Secret))
		h.Write([]byte(stringToSign))
		signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

		finalURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", config.URL, timestamp, signature)
	}

	if config.Type == "feishu" && config.Secret != "" {
		timestamp := time.Now().Unix()
		sign, err := xnet.GenSign(config.Secret, timestamp)
		if err != nil {
			return fmt.Errorf("飞书加签错误%v", err)
		}
		payload["timestamp"] = fmt.Sprintf("%d", timestamp)
		payload["sign"] = sign
	}

	fmt.Println(config.URL, config.Type, "发送内容为：", payload)

	body, _ := json.Marshal(payload)

	// 3. 发送请求
	req, _ := http.NewRequest("POST", finalURL, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("推送失败，HTTP 状态码: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取返回内容错误: %v", err)
	}

	fmt.Println(config.URL, config.Type, "推送返回内容为：", string(data))
	return nil
}

// GetScheduleConfig 供前端调用以加载当前配置
func (s *DomainService) GetScheduleConfig() (model.ScheduleConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.Data.ScheduleConfig, nil
}

// GetDashboardStats 获取仪表盘综合统计数据
func (s *DomainService) GetDashboardStats() (DashboardStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := s.store.Data
	now := time.Now().Unix()

	// 初始化统计变量
	var (
		sslAlerts    int
		domainAlerts int
		expiredCount int
		totalAlerts  int
	)

	thresholdSeconds := int64(data.ScheduleConfig.NotifyThreshold * 24 * 3600)
	if thresholdSeconds <= 0 {
		thresholdSeconds = 7 * 24 * 3600
	}

	// 遍历域名数据进行分类统计
	for _, d := range data.Domains {
		isSslIssue := false
		isDomainIssue := false

		// --- 证书维度判断 ---
		// 预警：进入阈值 或 扫描报错
		if (d.SSL.Expiry > 0 && (d.SSL.Expiry-now) < thresholdSeconds) || d.SSL.Status == "error" {
			sslAlerts++
			isSslIssue = true
		}
		// 过期：状态明确为 expired
		if d.SSL.Status == "expired" {
			expiredCount++
			isSslIssue = true
		}

		// --- 域名维度判断 ---
		if (d.Whois.Expiry > 0 && (d.Whois.Expiry-now) < thresholdSeconds) || d.Whois.Status == "error" {
			domainAlerts++
			isDomainIssue = true
		}
		if d.Whois.Status == "expired" {
			// 如果证书和域名都过期，这里只记一次总过期数，或者你可以分开统计
			expiredCount++
			isDomainIssue = true
		}

		// --- 总资产预警数 (去重) ---
		// 只要证书或域名任一维度有风险/错误，该资产就计入 AlertCount
		if isSslIssue || isDomainIssue {
			totalAlerts++
		}
	}

	// 构造返回对象
	stats := DashboardStats{
		CurrentTotal:     len(data.Domains),
		MaxLimit:         MaxLimitDomainCount,
		AlertCount:       totalAlerts,  // 异常资产总数
		SslAlertCount:    sslAlerts,    // 证书异常明细
		DomainAlertCount: domainAlerts, // 域名异常明细
		ExpiredCount:     expiredCount,
		NotifyThreshold:  data.ScheduleConfig.NotifyThreshold,
		NotifyTime:       data.ScheduleConfig.NotifyTime,
		WebhookStatus:    make(map[string]bool),
		UpdatedFields:    make(map[string]bool),
	}

	// 4. Webhook 状态检查
	for name, webhook := range data.Webhooks {
		stats.WebhookStatus[name] = webhook.URL != ""
	}

	// 5. 扫描时间计算
	if data.ScheduleConfig.LastFullScanAt > 0 {
		stats.LastScanTime = time.Unix(data.ScheduleConfig.LastFullScanAt, 0).Format("2006-01-02 15:04")
		stats.NextScanTime = time.Unix(data.ScheduleConfig.LastFullScanAt+86400, 0).Format("2006-01-02 15:04")
	}

	// 6. 快照对比逻辑 (UpdatedFields)
	if s.lastStats != nil {
		// 只要任意报警指标变了，就标记 alert 字段更新
		alertChanged := s.lastStats.AlertCount != stats.AlertCount ||
			s.lastStats.SslAlertCount != stats.SslAlertCount ||
			s.lastStats.DomainAlertCount != stats.DomainAlertCount ||
			s.lastStats.ExpiredCount != stats.ExpiredCount

		stats.UpdatedFields["alert"] = alertChanged
		stats.UpdatedFields["quota"] = s.lastStats.CurrentTotal != stats.CurrentTotal
		stats.UpdatedFields["schedule"] = s.lastStats.LastScanTime != stats.LastScanTime

		// 通知配置对比
		notifyOk := maps.Equal(s.lastStats.WebhookStatus, stats.WebhookStatus)
		stats.UpdatedFields["notify"] = !notifyOk || s.lastStats.NotifyTime != stats.NotifyTime
	}

	// 7. 更新快照并返回
	s.updateSnapshot(stats)
	return stats, nil
}

func (s *DomainService) updateSnapshot(stats DashboardStats) {
	s.lastStats = &StatsSnapshot{
		CurrentTotal:     stats.CurrentTotal,
		AlertCount:       stats.AlertCount,
		SslAlertCount:    stats.SslAlertCount,
		DomainAlertCount: stats.DomainAlertCount,
		ExpiredCount:     stats.ExpiredCount,
		NotifyThreshold:  stats.NotifyThreshold,
		NotifyTime:       stats.NotifyTime,
		LastScanTime:     stats.LastScanTime,
		WebhookStatus:    stats.WebhookStatus,
	}
}

func (s *DomainService) SelectDomainFile() (string, error) {
	path, err := s.manager.App.Dialog.OpenFile().
		SetTitle("选择域名文件").AddFilter("Txt", "*.txt").
		PromptForSingleSelection()

	if err != nil {
		return "", err
	}

	return path, nil
}

func ValidateDomainForImport(domain string) (bool, string) {
	// 1. 正则初步过滤格式 (快速)
	if !isFormatValid(domain) {
		return false, "域名格式不正确"
	}

	// 2. DNS 检查 (确认域名已注册并解析)
	if !checkDNS(domain) {
		return false, "域名解析失败(不存在)"
	}

	// 3. 端口检查 (确认是否有 HTTPS 服务)
	// if !checkPort(domain) {
	// 	return false, "无法连接到 HTTPS 服务"
	// }

	return true, ""
}

func (s *DomainService) VerifyDomainFile(filePath string) (VerifyResult, error) {
	var result VerifyResult

	file, err := os.Open(filePath)
	if err != nil {
		return result, err
	}
	defer file.Close()

	currentCount := s.getTotalCount()

	if currentCount >= MaxLimitDomainCount {
		return result, errors.New("已超出配额")
	}

	result.CurrentTotal = currentCount
	result.RemainingQuota = max(MaxLimitDomainCount-currentCount, 0)

	// 用于简单的去重记录
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		domain := line
		port := 443

		if strings.Contains(line, ":") {
			line1 := strings.Split(line, ":")
			domain = line1[0]
			p, err := strconv.Atoi(line1[1])
			if err != nil {
				validateErrMsg := fmt.Sprintf("%s-%s", line, "端口无效")
				result.Invalid = append(result.Invalid, validateErrMsg)
				continue
			}

			if p != 443 {
				port = p
			}
		}

		ok, str := ValidateDomainForImport(domain)

		if !ok {
			validateErrMsg := fmt.Sprintf("%s-%s", line, str)
			result.Invalid = append(result.Invalid, validateErrMsg)
			continue
		}

		// if 数据库已存在
		if s.domainMap[domain] != nil {
			port1 := s.domainMap[domain].Port
			if port1 == 0 {
				port1 = 443
			}

			if port == port1 {
				duplicateErrMsg := fmt.Sprintf("%s-%s", line, "数据库已存在")
				result.Duplicate = append(result.Duplicate, duplicateErrMsg)
				continue
			}

		}

		sline := fmt.Sprintf("%s:%d", domain, port)
		if seen[sline] {
			duplicateErrMsg := fmt.Sprintf("%s-%s", line, "文件中重复")
			result.Duplicate = append(result.Duplicate, duplicateErrMsg)
			continue
		}

		seen[sline] = true
		result.Valid = append(result.Valid, line)
	}

	if len(result.Valid) > result.RemainingQuota {
		// 记录真正能导入的部分
		// 可以在前端提示：由于配额限制，仅为您导入前 X 条
		result.Valid = result.Valid[0:result.RemainingQuota]
	}

	return result, nil
}

func (s *DomainService) DownloadDomainTemplate() error {
	path, err := s.manager.App.Dialog.
		SaveFile().
		SetFilename("domains.txt").
		PromptForSingleSelection()
	if err != nil || path == "" {
		return nil
	}

	tmplContent := `www.91demo.top:443
lab.91demo.top:4433
blog.91demo.top`

	return os.WriteFile(path, []byte(tmplContent), 0644)

}

// TestWebhook 供前端调用的测试接口
func (s *DomainService) TestWebhook(platform string, m model.WebhookModel) error {
	msg := "【豆子域名管家】这是一条测试消息。如果您收到此消息，说明 Webhook 配置正确且通信正常。别忘了点击保存！"
	m.Type = platform
	return s.sendToWebhook(m, msg)
}

// 检查是否设置成功方法
//  Get-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" | Select-Object -Property "sslchecker"

func (s *DomainService) SetAutoStart(state bool) error {
	// 1. 显式判断操作系统
	if runtime.GOOS != "windows" {
		return fmt.Errorf("当前系统为 %s，自启动设置仅支持 Windows 平台", runtime.GOOS)
	} else {
		err := SetAutoStart(state)
		if err != nil {
			return err
		}
	}
	return nil
}

// SaveScheduleConfig 供前端调用以保存通知策略
func (s *DomainService) SaveScheduleConfig(config model.ScheduleConfig) error {
	// 1. 参数校验
	if config.EnableNotify && config.NotifyThreshold <= 0 {
		return errors.New("告警通知天数不能小于 1")
	}

	s.mu.Lock()
	// 2. 精确更新：只覆盖用户在 UI 上修改的字段
	// 这样可以保护 LastFullScanAt 和 LastNotifyDate 不被前端传回的空值覆盖
	s.store.Data.ScheduleConfig.NotifyTime = config.NotifyTime
	s.store.Data.ScheduleConfig.NotifyThreshold = config.NotifyThreshold
	s.store.Data.ScheduleConfig.EnableNotify = config.EnableNotify

	// 获取一份包含内部标记的完整配置副本，用于重启调度器
	fullConfig := s.store.Data.ScheduleConfig
	s.mu.Unlock()

	// 3. 持久化
	s.store.MarkDirty()
	if err := s.store.SaveNow(); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	// 4. 重启调度器 (使用完整的配置信息)
	s.restartNotificationScheduler(fullConfig)

	// 5. 通知 UI 刷新
	s.notifyConfigRefresh("schedule")

	return nil
}
