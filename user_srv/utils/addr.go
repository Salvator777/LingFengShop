package utils

// utils 包通常用于存放各种通用的工具类和函数
// 这些工具类和函数与具体的业务逻辑无关，但在多个地方可能会被重复使用
import (
	"net"
)

// 获取一个当前可用的 TCP 端口
func GetFreePort() (int, error) {
	// 将地址解析为 localhost:0，0表示让os自动分配一个可用的端口
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	// 监听解析后的地址，操作系统会分配一个可用的端口
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	// 返回该端口号
	return l.Addr().(*net.TCPAddr).Port, nil
}
