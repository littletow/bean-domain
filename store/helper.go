package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

// 获取用户家目录下路径，并确保目录存在
func HomePath(rel string) string {
	u, err := user.Current()
	if err != nil {
		return rel // fallback
	}
	full := filepath.Join(u.HomeDir, rel)
	dir := filepath.Dir(full)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0700)
	}
	return full
}

func GetBase64FromData(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func GetDataFromBase64(data string) []byte {
	b, _ := base64.StdEncoding.DecodeString(data)
	return b
}

// 加密存储
func EncryptStore(data []byte, password string) (*EncryptedStore, error) {
	key := GetDataFromBase64(password)
	iv, payload, err := Encrypt(data, key)
	if err != nil {
		return nil, err
	}
	return &EncryptedStore{
		V:       currentStoreVersion,
		Alg:     "AES-GCM",
		IV:      iv,
		Payload: payload,
	}, nil
}

// 解密存储
func DecryptStore(enc *EncryptedStore, password string) ([]byte, error) {
	key := GetDataFromBase64(password)

	return Decrypt(enc.IV, enc.Payload, key)
}

// 派生密码
func DeriveKey(password string, salt []byte) []byte {
	// Argon2id 参数（保守、安全）
	const (
		time    = 3
		memory  = 64 * 1024 // 64 MB
		threads = 4
		keyLen  = 32
	)

	return argon2.IDKey(
		[]byte(password),
		salt,
		time,
		memory,
		threads,
		keyLen,
	)
}

// 生成盐
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	return salt, err
}

// 生成一个适合 AES-GCM 的随机 IV（12字节）
func GenerateIV() ([]byte, error) {
	iv := make([]byte, 12)
	_, err := rand.Read(iv)
	return iv, err
}

// 加密函数
func Encrypt(plaintext []byte, key []byte) (iv string, ciphertext string, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	ivBytes, err := GenerateIV()
	if err != nil {
		return "", "", err
	}

	ciphertextBytes := gcm.Seal(nil, ivBytes, plaintext, nil)

	iv = GetBase64FromData(ivBytes)
	ciphertext = GetBase64FromData(ciphertextBytes)
	err = nil
	return
}

// 解密函数
func Decrypt(encodedIV string, encodedCiphertext string, key []byte) ([]byte, error) {
	iv := GetDataFromBase64(encodedIV)
	ciphertext := GetDataFromBase64(encodedCiphertext)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// 这是一个标准的文件拷贝实现
func CopyFile(src, dst string) error {
	// 1. 打开源文件
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// 2. 创建目标文件
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// 3. 在内核空间进行数据拷贝
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// 4. 确保物理落盘
	return destFile.Sync()
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
