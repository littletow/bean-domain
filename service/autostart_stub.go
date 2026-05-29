//go:build !windows

package service

import "fmt"

func SetAutoStart(enable bool) error {
	return fmt.Errorf("抱歉，自启动功能暂不支持当前操作系统")
}

func IsAutoStartEnabled() bool {
	return false
}
