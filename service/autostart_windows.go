//go:build windows

package service

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "beandomain" // 进程名称

// SetAutoStart 仅在 Windows 下生效
func SetAutoStart(enable bool) error {

	// 打开当前用户的注册表启动项
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enable {
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		// 设置注册表键值，建议路径加引号以兼容包含空格的目录
		return k.SetStringValue(appName, `"`+execPath+`"`)
	} else {
		// 删除注册表键值
		err := k.DeleteValue(appName)
		if err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
}

// IsAutoStartEnabled 检查注册表中是否存在启动项
func IsAutoStartEnabled() bool {

	// 以只读权限打开注册表
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	// 尝试获取该键值的内容
	_, _, err = k.GetStringValue(appName)

	// 如果 err 为 nil，说明找到了配置，返回 true
	return err == nil
}
