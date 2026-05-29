package store

import (
	"bean-domain/model"
	"errors"
	"path/filepath"

	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/denisbrodbeck/machineid"
)

/**
* 进行了重构，不再兼容1.0 版本加密文件。
* 这里将域名的信息保存到文件，加密保存，需要做好数据结构变更兼容
* 密码由前端交互获得，使用派生密钥
* 生成两个文件，分别是key.json 和 data.json
* key.json 保存密钥信息，为了解决每次启动都输入密码，不是为了解决破解问题。
* data.json 保存域名配置等信息，添加了版本兼容。

* key.json 文件内容：
{
	"salt":"盐",
	"iv":"IV",
	"data":"加密的密钥"
}

* data.json 文件内容：
{
	"v":"1",
	"alg":"aes-gcm",
	"iv":"IV",
	"payload":"加密的域名数据"
}

* 当数据结构变更时，需要在migrate中做好处理，以兼容2.0版本开始的存储结构变更。
*/

// 当前存储结构体的版本号
const currentStoreVersion = 1

// 文件常量
const (
	keyFileName  = "key.json"  // 存放密钥
	dataFileName = "data.json" // 存放加密数据
)

// key文件数据
type keyFileData struct {
	Salt string `json:"salt"` // 盐
	IV   string `json:"iv"`   // IV
	Data string `json:"data"` // 密钥
}

// 加密存储结构
type EncryptedStore struct {
	V       int    `json:"v"`       // 版本号
	Alg     string `json:"alg"`     // 算法
	IV      string `json:"iv"`      // IV
	Payload string `json:"payload"` // 加密数据 base64(encrypt(json))
}

// 数据管理
type DataStore struct {
	mu       sync.RWMutex
	filePath string
	Data     model.Store
	isDirty  bool
	password string // 基于主密码的派生密码
}

// // 升级到最新版本
// func migrateV1(v1 StoreV1) Store {
// 	return Store{
// 		Domains:        v1.Domains,
// 		Webhooks:       v1.Webhooks,
// 		ScheduleConfig: v1.ScheduleConfig,
// 		AppState:       v1.AppState,
// 	}
// }

func defaultStore() model.Store {
	return model.Store{
		Domains: []model.DomainModel{},
		Webhooks: map[string]model.WebhookModel{
			"dingtalk": {},
			"wechat":   {},
			"feishu":   {},
		},
		ScheduleConfig: DefaultScheduleConfig(),
		AppState:       DefaultAppState(),
	}
}

// 默认调度
func DefaultScheduleConfig() model.ScheduleConfig {
	return model.ScheduleConfig{
		EnableNotify:    false,
		NotifyThreshold: 30,
		NotifyTime:      "10:00",
	}
}

func DefaultAppState() model.AppState {
	return model.AppState{}
}

func NewDataStore(path string) *DataStore {
	ds := &DataStore{
		filePath: path,
		Data:     defaultStore(),
	}
	return ds
}

func (ds *DataStore) ExistsKeyFile() bool {
	file := ds.getKeyFilePath()
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

func (ds *DataStore) ExistsDataFile() bool {
	file := ds.getDataFilePath()
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

func (ds *DataStore) getKeyFilePath() string {
	keyFilePath := filepath.Join(ds.filePath, keyFileName)
	return keyFilePath
}

func (ds *DataStore) getDataFilePath() string {
	dataFilePath := filepath.Join(ds.filePath, dataFileName)
	return dataFilePath
}

// 每隔x秒检查一次是否需要保存
func (ds *DataStore) autoSaveLoop() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		ds.save()
	}
}

// Close 供应用退出时调用，确保所有缓存数据落盘
func (ds *DataStore) Close() {
	err := ds.save()
	if err != nil {
		fmt.Printf("⚠️ 程序关闭前自动保存失败: %v\n", err)
	} else {
		fmt.Println("💾 程序退出，数据已安全保存")
	}
}

// MarkDirty 标记数据已变动，需要持久化
func (ds *DataStore) MarkDirty() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.isDirty = true
}

// SaveNow 强制立即将内存数据同步到磁盘
func (ds *DataStore) SaveNow() error {
	return ds.save()
}

// 数据文件保存
func (ds *DataStore) save() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if !ds.isDirty {
		return nil
	}

	plain, err := json.Marshal(ds.Data)
	if err != nil {
		return err
	}

	enc, err := EncryptStore(plain, ds.password)
	if err != nil {
		return err
	}

	data, err := json.Marshal(enc)
	if err != nil {
		return err
	}

	dataFilePath := ds.getDataFilePath()
	if err := atomicWrite(dataFilePath, data); err != nil {
		return err
	}

	ds.isDirty = false
	return nil
}

func (ds *DataStore) getMachineID() string {
	rawID, err := machineid.ProtectedID("SSLChecker")
	if err != nil {
		// 获取设备ID时失败，幽灵行为，返回一个默认值
		fmt.Println("💥 获取设备ID时错误，", err)
		return "SSLCheckerWillBeBetter!!!"
	}
	return rawID
}

// key文件创建
func (ds *DataStore) createKeyFile(password string) (string, error) {
	keyFileName := ds.getKeyFilePath()
	saltKey, err := GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("生成Salt错误：%v", err)
	}

	rawID := ds.getMachineID()
	// 生成密码的派生密码
	masterPassword := DeriveKey(password, saltKey)
	// 生成设备ID的派生密码
	devicePassword := DeriveKey(rawID, saltKey)

	iv, data, err := Encrypt(masterPassword, devicePassword)
	if err != nil {
		return "", fmt.Errorf("生成加密值错误：%v", err)
	}

	saltB64 := GetBase64FromData(saltKey)

	m := keyFileData{
		Salt: saltB64,
		IV:   iv,
		Data: data,
	}

	jsonData, err := json.Marshal(&m)
	if err != nil {
		return "", fmt.Errorf("JSON序列化错误：%v", err)
	}

	// 原子写入
	if err := atomicWrite(keyFileName, jsonData); err != nil {
		return "", err
	}

	passwordB64 := GetBase64FromData(masterPassword)

	return passwordB64, nil
}

// 初始化（首次启动）
func (ds *DataStore) Init(password string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	// 创建KEY文件
	password1, err := ds.createKeyFile(password)
	if err != nil {
		return fmt.Errorf("创建KEY文件失败，%v", err)
	}

	ds.password = password1
	// 创建数据文件
	plain, err := json.Marshal(ds.Data)
	if err != nil {
		return err
	}

	enc, err := EncryptStore(plain, ds.password)
	if err != nil {
		return err
	}

	data, err := json.Marshal(enc)
	if err != nil {
		return err
	}

	dataFilePath := ds.getDataFilePath()
	if err := atomicWrite(dataFilePath, data); err != nil {
		return err
	}

	return nil
}

// 加载KEY文件
func (ds *DataStore) loadKeyFile() (string, error) {
	hasExist := ds.ExistsKeyFile()
	if !hasExist {
		return "", errors.New("幽灵行为：文件不存在或已删除") // 文件不存在，跳过加载
	}

	keyFileName := ds.getKeyFilePath()
	// 尝试加载已有 salt
	fileData, err := os.ReadFile(keyFileName)
	if err != nil {
		// 损坏的key文件
		return "", fmt.Errorf("读取文件错误：%v", err)
	}

	var data keyFileData
	err = json.Unmarshal(fileData, &data)
	if err != nil {
		return "", fmt.Errorf("JSON序列化错误：%v", err)
	}

	rawID := ds.getMachineID()
	salt := GetDataFromBase64(data.Salt)

	masterKey := DeriveKey(rawID, salt)
	keyData, err := Decrypt(data.IV, data.Data, masterKey)
	if err != nil {
		return "", fmt.Errorf("解密KEY值错误：%v", err)
	}

	keyDataB64 := GetBase64FromData(keyData)

	return keyDataB64, nil
}

// 加载（非首次启动），加载数据内容，如果有数据结构变更，请先定义旧结构体，然后在这里进行兼容，并增加版本号
func (ds *DataStore) Load() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// 加载KEY文件
	password, err := ds.loadKeyFile()
	if err != nil {
		// 只有用户删除了KEY文件或者逻辑漏洞
		fmt.Println("💥 获取KEY文件时错误，", err)
		return fmt.Errorf("加载KEY文件失败，%v", err)
	}

	ds.password = password
	dataFileName := ds.getDataFilePath()
	data, err := os.ReadFile(dataFileName)
	if err != nil {
		// 只有用户删除了数据文件或者逻辑漏洞
		fmt.Println("💥 获取数据文件时错误，", err)
		return err
	}

	var enc EncryptedStore
	if err := json.Unmarshal(data, &enc); err != nil {
		return err
	}

	plain, err := DecryptStore(&enc, password)
	if err != nil {
		return err
	}

	err = ds.migrate(enc.V, plain)
	if err != nil {
		return err
	}

	// 加载自动保存
	go ds.autoSaveLoop()

	return nil
}

func (ds *DataStore) migrate(version int, plain []byte) error {
	switch version {
	// case 1:
	// 	var v1 StoreV1
	// 	json.Unmarshal(plain, &v1)
	// 	ds.Data = migrateV1(v1)

	default:
		return json.Unmarshal(plain, &ds.Data)
	}

}

// TODO 微信扫码备份逻辑，用户扫码之后获取openid，然后生成派生密码，提前主密码的派生密码，然后提取数据文件，加密为备份文件。
// 复制到另一台电脑进行恢复，再次使用扫码逻辑，基于相同的逻辑在新的电脑上生成新的数据文件。
