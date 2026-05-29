package util

import (
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

func GenDeviceID() string {
	// 已剔除: I, O, S, Z
	const charset = "ABCDEFGHJKLMNPQRTUVWXY0123456789"
	const length = 6

	result := make([]byte, length)
	for i := 0; i < length; i++ {
		// 使用 crypto/rand 生成真随机索引
		num, err := crand.Int(crand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// 退极情况：如果随机源失效，使用时间戳兜底（基本不会发生）
			result[i] = charset[0]
			continue
		}
		result[i] = charset[num.Int64()]
	}

	// 返回格式如: B01-K7PXMR (加个连字符更易读) 或 B01K7PXMR
	return fmt.Sprintf("B01-%s", string(result))
}

// HmacSign 生成 HMAC-SHA256 签名，并返回十六进制编码的字符串
func HmacSign(data string, key string) string {
	// 1. 创建一个新的 HMAC 哈希器。
	h := hmac.New(sha256.New, []byte(key))

	// 2. 将数据写入哈希器。
	h.Write([]byte(data))

	// 3. 计算最终的 HMAC 签名（消息摘要）。
	signature := h.Sum(nil)

	// 4. 将签名结果（[]byte）编码为十六进制字符串。
	encodedSignature := hex.EncodeToString(signature)

	return encodedSignature
}

func FormatTimestamp(expiry int64) string {
	if expiry == 0 {
		return ""
	}
	// 1. 将 int64 转换为 time.Time (假设是秒级时间戳)
	t := time.Unix(expiry, 0)

	// 2. 格式化为 YYYY-MM-DD
	// Go 的布局字符串必须是这个特定的日期
	return t.Format("2006-01-02")
}

// 如果你的时间戳是毫秒级的（如 JavaScript 生成的）：
func FormatMilliTimestamp(expiry int64) string {
	if expiry == 0 {
		return ""
	}
	t := time.UnixMilli(expiry)
	return t.Format("2006-01-02")
}
