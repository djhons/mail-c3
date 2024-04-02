package mail

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"gopkg.in/gomail.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// 定义邮件配置信息结构体
type MailConfig struct {
	SMTPServer  string
	SMTPPort    int
	IMAPServer  string
	IMAPPort    int
	Username    string
	Password    string
	SenderEmail string
	CheckTime   time.Duration
}

func NewMail(mailConfig MailConfig) *MailConfig {
	return &mailConfig
}
func decodeAndDecompress(encodedData string) ([]byte, error) {
	// 使用base64解码数据
	decodedData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, err
	}

	// 使用bytes.Buffer包装解码后的数据
	buf := bytes.NewBuffer(decodedData)

	// 创建gzip读取器
	gzipReader, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	// 读取解压后的数据
	var decompressedData bytes.Buffer
	_, err = decompressedData.ReadFrom(gzipReader)
	if err != nil {
		return nil, err
	}

	return decompressedData.Bytes(), nil
}

// 发送邮件
func (email *MailConfig) Send(msg string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", email.Username)
	m.SetHeader("To", email.SenderEmail) // 收件人，可以多个收件人，但必须使用相同的 SMTP 连接
	m.SetHeader("Subject", "Hello!")     // 邮件主题

	m.SetBody("text/html", msg)

	d := gomail.NewDialer(email.SMTPServer, email.SMTPPort, email.Username, email.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	if err := d.DialAndSend(m); err != nil {
		log.Println("send mail error")
	} else {
		log.Println("send mail success")
	}
	return nil
}

// 接收邮件
func (email MailConfig) Receive(msgCh chan []byte) error {
	imap.CharsetReader = charset.Reader
	log.Println("Connecting to imap server...", email.IMAPServer)
	c, err := client.DialTLS(fmt.Sprintf("%s:%d", email.IMAPServer, email.IMAPPort), nil)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer c.Logout()
	log.Println("Connected")
	if err := c.Login(email.Username, email.Password); err != nil {
		return err
	}
	log.Println("Logged in")

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
		return err
	}

	var s imap.BodySectionName
	// Keep track of initial message count
	initialMsgCount := mbox.Messages
	for {
		// Refresh mailbox status
		mbox, err := c.Select("INBOX", false)
		if err != nil {
			log.Fatal(err)
		}
		if mbox.Messages > initialMsgCount {
			log.Println("new mail")
			seqset := new(imap.SeqSet)
			seqset.AddRange(initialMsgCount+1, mbox.Messages)
			//messages := make(chan *imap.Message, mbox.Messages)
			//done := make(chan error, 1)
			//go func() {
			//	done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
			//}()
			chanMessage := make(chan *imap.Message, 2)
			go func() {
				// 获取邮件MIME内容
				if err = c.Fetch(seqset,
					[]imap.FetchItem{imap.FetchRFC822},
					chanMessage); err != nil {
					log.Println(seqset, err)
				}
			}()
			for msg := range chanMessage {
				r := msg.GetBody(&s)
				if r == nil {
					log.Fatal("Server didn't return message body")
				}
				mr, err := mail.CreateReader(r)
				if err != nil {
					log.Fatal(err)
				}
				for {
					p, err := mr.NextPart()
					if err == io.EOF {
						break
					} else if err != nil {
						log.Fatal(err)
						break
					}
					// Print message body if it's from the specified sender
					mailfrom, err := mr.Header.AddressList("From")
					if mailfrom[0].Address == email.SenderEmail {
						switch h := p.Header.(type) {
						case *mail.InlineHeader:
							// 获取正文内容, text或者html
							mailContent, _ := ioutil.ReadAll(p.Body)
							mailContent, err = decodeAndDecompress(string(mailContent[67:]))
							msgCh <- mailContent
							break
						case *mail.AttachmentHeader:
							// 下载附件
							filename, err := h.Filename()
							if err != nil {
								log.Fatal(err)
							}
							if filename != "" {
								log.Println("Got attachment: ", filename)
								b, _ := ioutil.ReadAll(p.Body)
								file, _ := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, os.ModePerm)
								defer file.Close()
								n, err := file.Write(b)
								if err != nil {
									fmt.Println("写入文件异常", err.Error())
								} else {
									fmt.Println("写入Ok：", n)
								}
							}
						}
					}
				}
			}
			initialMsgCount = mbox.Messages
		}

		time.Sleep(email.CheckTime * time.Second) // Check every 30 seconds
	}
	return nil
}
