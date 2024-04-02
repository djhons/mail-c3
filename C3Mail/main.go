package main

import (
	"C3Mail/mail"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"github.com/go-ini/ini"
	"log"
	"net"
)

type Config struct {
	MailConfig mail.MailConfig
	ServerPort int
}

// 从INI格式的配置文件中加载邮件配置信息
func loadConfig(filename string) (Config, error) {
	var config Config

	// 加载INI格式的配置文件
	cfg, err := ini.Load(filename)
	if err != nil {
		return config, err
	}

	// 解析配置文件中的各项配置
	err = cfg.Section("config").MapTo(&config)
	if err != nil {
		return config, err
	}
	return config, nil
}
func compressAndEncode(data []byte) (string, error) {
	// 使用bytes.Buffer作为中间缓冲区
	var b bytes.Buffer

	// 创建gzip写入器
	gzipWriter := gzip.NewWriter(&b)

	// 写入数据到gzip写入器
	_, err := gzipWriter.Write(data)
	if err != nil {
		return "", err
	}

	// 关闭gzip写入器
	err = gzipWriter.Close()
	if err != nil {
		return "", err
	}

	// 使用base64编码压缩后的数据
	encodedData := base64.StdEncoding.EncodeToString(b.Bytes())

	return encodedData, nil
}

func handleConnection(conn net.Conn, msgCh chan []byte, mailObj *mail.MailConfig) {
	defer conn.Close()
	buffer := make([]byte, 1024*1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading:", err.Error())
		return
	}
	send, err := compressAndEncode(buffer[:n])
	if err != nil {
		return
	}
	mailObj.Send(send)
	select {
	case msg := <-msgCh:
		log.Println(string(msg))
		conn.Write(msg)
	default:
		response := "HTTP/1.1 200 OK\r\n" +
			"Content-Type: text/plain\r\n" +
			"Connection: close\r\n\r\n" +
			"Hello, this is a simple HTTP server!\r\n"
		_, err = conn.Write([]byte(response))
		if err != nil {
			return
		}
	}
}
func main() {
	config, err := loadConfig("config.ini")
	if err != nil {
		log.Panicln("[-] ", err)
	}
	newMailCh := make(chan []byte)
	mailObj := mail.NewMail(config.MailConfig)
	go mailObj.Receive(newMailCh)
	//for {
	//	newMail := <-newMailCh
	//	fmt.Println("New email received:", string(newMail))
	//}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.ServerPort))
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		return
	}
	defer listener.Close()

	fmt.Println("Server listening on port", config.ServerPort)

	for {
		// 等待客户端连接
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err.Error())
			return
		}

		// 处理连接
		go handleConnection(conn, newMailCh, mailObj)
	}
}
