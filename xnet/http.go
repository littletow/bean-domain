package xnet

import (
	"bean-domain/model"
	"bean-domain/util"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/quic-go/quic-go"
	"golang.org/x/net/publicsuffix"
)

/**
* 这个文件用来检测域名信息。
*
 */

var (
	ticket     string
	ticketTime int64
	devID      string
	mpcode     string
)

type checkResult struct {
	expireTime int64
	err        error
}

type apiResp struct {
	Code  int             `json:"code"`
	Msg   string          `json:"msg"`
	Data  json.RawMessage `json:"data,omitempty"`
	Count int             `json:"count,omitempty"`
}

// GenerateSignature 生成签名：拼接 Content + Timestamp
func generateSignature(content, ts string) string {
	const hmackey = "xv0xetrtWYvggelrJiMh6wpjNKi0CswT" // 必须与服务端一致

	// 拼接逻辑必须与服务端完全一致
	payload := fmt.Sprintf("%s%s", content, ts)

	h := hmac.New(sha256.New, []byte(hmackey))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func whoisCheckV1(domain string) (int64, error) {
	// 1. 自动提取主域名 (例如 lab.91demo.top -> 91demo.top)
	mainDomain, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return 0, fmt.Errorf("无效域名格式: %v", err)
	}

	// 2. 查询原始 Whois 信息 (必须查主域名)
	raw, err := whois.Whois(mainDomain)
	if err != nil {
		return 0, fmt.Errorf("whois 查询失败: %v", err)
	}

	// 3. 解析结果
	result, err := whoisparser.Parse(raw)
	if err != nil {
		return 0, fmt.Errorf("解析失败: %v", err)
	}

	// 4. 获取到期时间
	expiryStr := result.Domain.ExpirationDate
	if expiryStr == "" {
		return 0, fmt.Errorf("未找到到期日期")
	}

	// 5. 增强的时间解析
	// whoisparser 通常已经尝试将时间标准化，但为了稳妥，
	// 我们可以尝试解析解析器返回的标准格式
	t, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		// 如果 RFC3339 失败，尝试 whoisparser 内部转换后的常见格式
		t, err = time.Parse("2006-01-02 15:04:05", expiryStr)
		if err != nil {
			return 0, fmt.Errorf("无法解析日期格式: %s", expiryStr)
		}
	}

	return t.Unix(), nil
}

func parseFlexibleDate(dateStr string) (time.Time, error) {
	// 预定义可能出现的多种时间格式
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"02-Jan-2006", // 某些国别后缀常用
		"02-01-2006",  // .hk 常用格式
		"2006-01-02T15:04:05Z",
		"January 02, 2006",
	}

	var lastErr error
	for _, f := range formats {
		if t, err := time.Parse(f, strings.TrimSpace(dateStr)); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

func WhoisCheck(domain string) (int64, error) {
	mainDomain, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return 0, err
	}

	raw, err := whois.Whois(mainDomain)
	if err != nil {

		return 0, err
	}

	result, err := whoisparser.Parse(raw)
	if err != nil {
		// 如果 likexian 解析器失败，尝试使用 icamys/whois-parser 作为备选
		// 或者直接报错，提示需要手动更新解析规则

		return 0, err
	}

	expiryStr := result.Domain.ExpirationDate
	if expiryStr == "" {

		return 0, fmt.Errorf("未找到到期日期")
	}

	// 使用增强的多格式解析
	t, err := parseFlexibleDate(expiryStr)
	if err != nil {
		errMsg := fmt.Sprintf("无法解析日期: %s (%v)", expiryStr, err)
		return 0, errors.New(errMsg)
	}

	return t.Unix(), nil
}

// 原始tcp版本
// func sslCheck(domain string, port int) (int64, error) {
// 	// 1. 定义拨号器并设置超时
// 	dialer := &net.Dialer{
// 		Timeout: 5 * time.Second,
// 	}

// 	// 2. 直接建立 TLS 连接（跳过证书链验证以检查过期证书）
// 	// 注意：此处地址必须包含端口
// 	conn, err := tls.DialWithDialer(dialer, "tcp", domain+":443", &tls.Config{
// 		ServerName:         domain,
// 		InsecureSkipVerify: true, // 允许获取已过期或自签名的证书信息
// 	})

// 	if err != nil {
// 		// 打印具体的错误类型，看是 timeout、EOF 还是具体的 tls: handshake failure

// 		return 0, err
// 	}
// 	defer conn.Close()

// 	// 3. 获取连接状态中的证书链
// 	state := conn.ConnectionState()
// 	if len(state.PeerCertificates) == 0 {
// 		errMsg := fmt.Sprintf("no certificates found for %s", domain)

// 		return 0, errors.New(errMsg)
// 	}

// 	// 4. 获取第一张证书（端实体证书）的过期时间
// 	cert := state.PeerCertificates[0]
// 	return cert.NotAfter.Unix(), nil
// }

func SSLCheck(domain string, port int) (int64, error) {
	if port == 0 {
		port = 443
	}
	addr := fmt.Sprintf("%s:%d", domain, port)

	// 控制整体超时，防止协程永久挂起
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resultChan := make(chan checkResult, 2)
	var wg sync.WaitGroup

	// 1. 发起 TCP 检测
	wg.Go(func() {
		t, err := checkByTCP(ctx, domain, addr)
		resultChan <- checkResult{t, err}
	})

	// 2. 发起 UDP (QUIC) 检测
	wg.Go(func() {
		t, err := checkByQUIC(ctx, domain, addr)
		resultChan <- checkResult{t, err}
	})

	// 辅助协程：当所有任务结束时关闭 channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var lastErr error
	// 3. 收集结果
	for range 2 {
		res := <-resultChan
		if res.err == nil {
			cancel() // 一旦有一个成功，立即取消另一个正在进行的请求
			return res.expireTime, nil
		}
		lastErr = res.err // 记录错误，如果都失败了就返回最后一个错误
	}

	return 0, fmt.Errorf("all protocols failed: %v", lastErr)
}

func checkByTCP(ctx context.Context, domain, addr string) (int64, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}

	// 使用 DialWithDialer 但需配合 context 处理取消
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName:         domain,
		InsecureSkipVerify: true,
	})
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// 检查 Context 是否已取消
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		cert := conn.ConnectionState().PeerCertificates[0]
		return cert.NotAfter.Unix(), nil
	}
}

func checkByQUIC(ctx context.Context, domain, addr string) (int64, error) {
	tlsConf := &tls.Config{
		ServerName:         domain,
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}

	conn, err := quic.DialAddr(ctx, addr, tlsConf, &quic.Config{
		HandshakeIdleTimeout: 5 * time.Second,
		MaxIdleTimeout:       5 * time.Second,
	})
	if err != nil {
		return 0, err
	}
	defer conn.CloseWithError(0, "")

	certs := conn.ConnectionState().TLS.PeerCertificates
	if len(certs) == 0 {
		return 0, fmt.Errorf("no quic certs")
	}

	c := certs[0]

	fmt.Printf("域名: %v\n", c.Subject.CommonName)
	fmt.Printf("序列号: %X\n", c.SerialNumber) // 对比这个序列号！
	fmt.Printf("有效期至: %v\n", c.NotAfter.Format("2006-01-02 15:04:05"))
	return certs[0].NotAfter.Unix(), nil
}

// func sslCheck(domain string, port int) (int64, error) {
// 	// 如果传入的 port 为 0，默认使用 443
// 	if port == 0 {
// 		port = 443
// 	}
// 	addr := fmt.Sprintf("%s:%d", domain, port)

// 	var tcpErr, udpErr error
// 	var expireTime int64

// 	// ==========================================
// 	// 1. 尝试常规 TCP + TLS 检测
// 	// ==========================================
// 	expireTime, tcpErr = checkByTCP(domain, addr)
// 	if tcpErr == nil {
// 		return expireTime, nil // TCP 成功，直接返回
// 	}

// 	// ==========================================
// 	// 2. TCP 失败，尝试 UDP + QUIC 检测
// 	// ==========================================
// 	expireTime, udpErr = checkByQUIC(domain, addr)
// 	if udpErr == nil {
// 		return expireTime, nil // QUIC 成功，直接返回
// 	}

// 	// ==========================================
// 	// 3. 两者都失败，组装错误返回
// 	// ==========================================
// 	return 0, fmt.Errorf("both TCP and UDP failed. TCP Err: %v; UDP Err: %v", tcpErr, udpErr)
// }

// checkByTCP 执行常规的 TCP TLS 握手
// func checkByTCP(domain, addr string) (int64, error) {
// 	dialer := &net.Dialer{
// 		Timeout: 5 * time.Second,
// 	}

// 	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
// 		ServerName:         domain,
// 		InsecureSkipVerify: true, // 允许获取过期或自签名证书
// 	})
// 	if err != nil {
// 		return 0, err
// 	}
// 	defer conn.Close()

// 	state := conn.ConnectionState()
// 	if len(state.PeerCertificates) == 0 {
// 		return 0, fmt.Errorf("no certificates found via TCP")
// 	}

// 	return state.PeerCertificates[0].NotAfter.Unix(), nil
// }

// checkByQUIC 执行 UDP QUIC 握手获取证书
// func checkByQUIC(domain, addr string) (int64, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	tlsConf := &tls.Config{
// 		ServerName:         domain,
// 		InsecureSkipVerify: true,
// 	}

// 	// 仅建立 QUIC 连接，不打开 H3 stream
// 	conn, err := quic.DialAddr(ctx, addr, tlsConf, &quic.Config{
// 		MaxIdleTimeout: 5 * time.Second,
// 	})
// 	if err != nil {
// 		return 0, err
// 	}
// 	defer conn.CloseWithError(0, "")

// 	state := conn.ConnectionState()
// 	if len(state.TLS.PeerCertificates) == 0 {
// 		return 0, fmt.Errorf("no certificates found via QUIC")
// 	}

// 	return state.TLS.PeerCertificates[0].NotAfter.Unix(), nil
// }

func WechatNotify(url string, content string) {
	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Println("发送报表编码错误，", err)
	}
	rsp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Println("发送报表请求错误，", err)
	}
	defer rsp.Body.Close()
	result, err := io.ReadAll(rsp.Body)
	if err != nil {
		log.Println("发送报表读取数据错误，", err)
	}
	log.Println("发送报表成功", string(result))
}

func DingtalkNotify(webhookURL string, content string, secret string) {
	finalURL := webhookURL

	// 如果配置了 Secret，执行加签逻辑
	if secret != "" {
		timestamp := time.Now().UnixNano() / 1e6
		stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)

		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(stringToSign))
		signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

		finalURL = fmt.Sprintf("%s&timestamp=%d&sign=%s",
			webhookURL, timestamp, url.QueryEscape(signature))
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}

	body, _ := json.Marshal(payload)
	http.Post(finalURL, "application/json", bytes.NewBuffer(body))
}

func FeishuNotify(webhookURL string, content string, secret string) {
	finalURL := webhookURL
	payload := make(map[string]interface{})
	payload["msg_type"] = "text"
	payload["content"] = map[string]string{
		"text": content, // 这里只能传入纯字符串
	}

	// 如果配置了 Secret，执行加签逻辑
	if secret != "" {
		timestamp := time.Now().Unix()
		// 1. 计算签名 (timestamp + "\n" + secret)
		sign, err := GenSign(secret, timestamp)
		if err != nil {
			log.Println("飞书加签错误，", err)
			return
		}
		payload["timestamp"] = fmt.Sprintf("%d", timestamp)
		payload["sign"] = sign
	}

	body, _ := json.Marshal(payload)
	http.Post(finalURL, "application/json", bytes.NewBuffer(body))
}

func GenSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	// 注意：飞书文档要求将 timestamp + \n + secret 作为整体进行签名
	_, err := h.Write([]byte(""))
	if err != nil {
		return "", err
	}
	// 实际 HMAC 逻辑在 Sum 中完成
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func GetRemoteVersion() (model.UpdateInfo, error) {
	var info model.UpdateInfo
	// 下载最新版本相关信息
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest("GET", "https://client.91demo.top/c/cver?id=domain", nil)
	if err != nil {
		fmt.Println("[API调试] 创建请求错误，", err)
		return info, err
	}
	req.Header.Set("X-Client", "BeanTool")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("[API调试] 发送请求错误，", err)
		return info, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[API调试] 读取数据错误，", err)
		return info, err
	}

	var result apiResp
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("[API调试] 解析数据错误1，", err)
		return info, err
	}

	err = json.Unmarshal(result.Data, &info)
	if err != nil {
		fmt.Println("[API调试] 解析数据错误2，", err)
		return info, err
	}
	fmt.Println("[API调试] 远端版本数据，", info.LatestVersion, info.DownloadUrl, info.UpdateLog)
	return info, nil
}

func isTicketValid() bool {
	now := time.Now().Unix()
	return now-ticketTime < 240
}

// 获取客户端票据
func getClientTicket(devID string) string {
	client := &http.Client{Timeout: 8 * time.Second}
	url := fmt.Sprintf("https://client.91demo.top/c/ticket?devID=%s&devType=domain", devID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("[API调试] 创建请求错误，", err)
		return ""
	}
	req.Header.Set("X-Client", "BeanTool")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("[API调试] 发送请求错误，", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[API调试] 读取数据错误，", err)
		return ""
	}
	fmt.Println("[API调试] 数据1", string(body))
	var result apiResp
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("[API调试] 解析数据错误1，", err)
		return ""
	}
	fmt.Println("[API调试] 数据2", result)
	err = json.Unmarshal(result.Data, &ticket)
	if err != nil {
		fmt.Println("[API调试] 解析数据错误2，", err)
		return ""
	}

	ticketTime = time.Now().Unix()
	fmt.Println("[API调试] Ticket数据，", ticket)
	return ticket
}

func doHttpGet(api string, params map[string]string) (json.RawMessage, error) {
	if devID == "" {
		devID = util.GenDeviceID()
	}

	ok := isTicketValid()
	if !ok {
		ticket = getClientTicket(devID)
	}

	if ticket == "" {
		return nil, errors.New("无法获取ticket")
	}

	baseURL := "https://client.91demo.top/c"

	u, err := url.Parse(baseURL + api)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	now := fmt.Sprintf("%d", time.Now().Unix())
	signStr := fmt.Sprintf("%s:%s", devID, now)
	sign := util.HmacSign(signStr, ticket)
	req.Header.Set("X-DevID", devID)
	req.Header.Set("X-DevType", "domain")
	req.Header.Set("X-TS", now)
	req.Header.Set("X-SIGN", sign)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result apiResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != 1 {
		return nil, fmt.Errorf("%s", result.Msg)
	}

	return result.Data, nil
}

// func doHttpPost(api string, bodyData any) (json.RawMessage, error) {
// 	url := "https://client.91demo.top" + api

// 	b, err := json.Marshal(bodyData)
// 	if err != nil {
// 		return nil, err
// 	}

// 	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
// 	if err != nil {
// 		return nil, err
// 	}

// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("X-Client", "BeanTool")

// 	client := &http.Client{Timeout: 8 * time.Second}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var result apiResp
// 	if err := json.Unmarshal(body, &result); err != nil {
// 		return nil, err
// 	}

// 	if result.Code != 0 {
// 		return nil, fmt.Errorf(result.Msg)
// 	}

// 	return result.Data, nil
// }

func GetMpCode() string {
	if devID == "" {
		devID = util.GenDeviceID()
	}

	if mpcode != "" {
		return mpcode
	}

	params := make(map[string]string)
	params["devID"] = devID
	params["devType"] = "domain"

	result, err := doHttpGet("/mpcode", params)
	if err != nil {
		fmt.Println("[API调试] 请求结果错误，", err)
		return ""
	}

	err = json.Unmarshal(result, &mpcode)
	if err != nil {
		fmt.Println("[API调试] 解析数据错误，", err)
		return ""
	}
	// fmt.Println("小程序码数据，", mpcode)
	return mpcode
}
