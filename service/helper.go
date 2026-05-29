package service

import (
	"context"
	"net"
	"regexp"
	"time"
)

// func generateDeviceID() string {
// 	// 已剔除: I, O, S, Z
// 	const charset = "ABCDEFGHJKLMNPQRTUVWXY0123456789"
// 	const length = 6

// 	result := make([]byte, length)
// 	for i := 0; i < length; i++ {
// 		// 使用 crypto/rand 生成真随机索引
// 		num, err := crand.Int(crand.Reader, big.NewInt(int64(len(charset))))
// 		if err != nil {
// 			// 退极情况：如果随机源失效，使用时间戳兜底（基本不会发生）
// 			result[i] = charset[0]
// 			continue
// 		}
// 		result[i] = charset[num.Int64()]
// 	}

// 	// 返回格式如: B01-K7PXMR (加个连字符更易读) 或 B01K7PXMR
// 	return fmt.Sprintf("B01-%s", string(result))
// }

func isFormatValid(domain string) bool {
	// 匹配标准的域名格式，不带协议头和端口
	pattern := `^([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$`
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(domain)
}

func checkDNS(domain string) bool {
	// 设置 2 秒超时，防止验证过程卡死
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 尝试获取 IP 地址
	_, err := net.DefaultResolver.LookupHost(ctx, domain)
	return err == nil
}

func checkPort(domain string) bool {
	// 尝试建立 TCP 连接，超时设为 2 秒
	address := net.JoinHostPort(domain, "443")
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}
