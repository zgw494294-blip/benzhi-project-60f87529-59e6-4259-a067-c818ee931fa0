package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
)

const defaultAddress = "127.0.0.1:19081"

func addressDefault() string {
	portText := os.Getenv("PORT")
	if portText == "" {
		return defaultAddress
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return defaultAddress
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func validateAddress(value string) error {
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("监听地址必须采用 host:port 格式: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("监听端口必须在 1 到 65535 之间")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return errors.New("监听地址不得为空或绑定所有网络接口")
	}
	return nil
}
