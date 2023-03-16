package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"main/config"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const dateFormat = "2006-01-02"

type BotConfig struct {
	BotToken string `json:"botToken"`
	ChatId   string `json:"chatId"`
}

var proxyAddr = flag.String("proxy", "", "proxy addr for request")

//go:generate go install github.com/hajimehoshi/file2byteslice
//go:generate mkdir -p config
//go:generate file2byteslice -input hosts.txt -output config/hosts.go -package config -var Hosts
//go:generate file2byteslice -input bot.json -output config/bot.go -package config -var Bot
func main() {
	flag.Parse()
	var hosts = string(config.Hosts[:])
	hostSlice := strings.Split(hosts, ",")
	var telegramMsg = "域名过期时间巡检:" + time.Now().Format(dateFormat) + "\n"
	telegramMsg += "----------------------------------------------\n"
	for _, host := range hostSlice {
		t, err := checkSsl(trim(host))
		if err != nil {
			telegramMsg += host + " 获取证书过期时间失败\n"
			continue
		}
		telegramMsg += host + " " + t.Format(dateFormat)
		if t.Before(time.Now()) {
			telegramMsg += "️❌"
		} else if t.Before(time.Now().Add(15 * time.Minute * 60 * 24)) {
			telegramMsg += "⚠️"
		} else {
			telegramMsg += "✔"
		}
		telegramMsg += "\r"
	}
	bot := BotConfig{}
	_ = json.Unmarshal(config.Bot[:], &bot)
	pUrl := "https://api.telegram.org/bot" + bot.BotToken + "/sendMessage"
	params := map[string]interface{}{}
	params["chat_id"] = bot.ChatId
	params["text"] = telegramMsg
	data, err := json.Marshal(params)
	if err != nil {
		fmt.Println("参数json化失败:" + err.Error())
		return
	}
	param := bytes.NewBuffer(data)
	var client http.Client
	if *proxyAddr != "" {
		proxy, err := url.Parse(*proxyAddr)
		if err != nil {
			fmt.Println("proxy 解析失败:" + err.Error())
			return
		}
		client = http.Client{
			Timeout: time.Second * 10,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
				Proxy:           http.ProxyURL(proxy),
			},
		}
	} else {
		client = http.Client{
			Timeout: 5 * time.Second, // 5秒最大超时
		}
	}
	resp, err := client.Post(pUrl, "application/json", param)
	if err != nil {
		fmt.Println("发起请求失败:" + err.Error())
		return
	}
	defer resp.Body.Close()
	//body, err := io.ReadAll(resp.Body)
	//if err != nil {
	//	fmt.Println("读取响应失败:" + err.Error())
	//	return
	//}
	//fmt.Println(string(body))
}

func checkSsl(host string) (*time.Time, error) {
	duration := time.Duration(30 * 1000 * 1000 * 1000)
	var dialer net.Dialer
	dialer.Timeout = duration
	addr := host + ":443"
	conn, err := tls.DialWithDialer(&dialer, "tcp", addr, &tls.Config{})
	if err != nil {
		return nil, err
	} else {
		state := conn.ConnectionState()
		certs := state.PeerCertificates
		cert := *certs[0]
		notAfter := cert.NotAfter.Local()
		defer conn.Close()
		return &notAfter, nil
	}
}

func trim(src string) (dist string) {
	if len(src) == 0 {
		return
	}
	var distR []rune
	r := []rune(src)
	for i := 0; i < len(r); i++ {
		if r[i] == 10 || r[i] == 32 {
			continue
		}
		distR = append(distR, r[i])
	}
	dist = string(distR)
	return
}
