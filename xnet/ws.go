package xnet

import (
	"bean-domain/util"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type WsMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

type Client struct {
	url  string
	conn *websocket.Conn
	ctx  context.Context
	stop context.CancelFunc

	result chan map[string]interface{}

	active int32 // 1=运行中 0=已主动关闭
}

func NewClient(url string) *Client {
	ctx, stop := context.WithCancel(context.Background())
	return &Client{
		url:    url,
		ctx:    ctx,
		stop:   stop,
		result: make(chan map[string]interface{}, 1),
		active: 1,
	}
}

func (c *Client) Start() {
	go c.connectLoop()
}

func (c *Client) connectLoop() {
	for {
		if atomic.LoadInt32(&c.active) == 0 {
			return
		}

		header := http.Header{}
		header.Set("Origin", "https://client.91demo.top")
		conn, _, err := websocket.DefaultDialer.Dial(c.url, header)
		if err != nil {
			log.Println("dial failed:", err)
			time.Sleep(3 * time.Second)
			continue
		}

		c.conn = conn
		log.Println("ws connected")

		c.run()

		// run 退出后判断是否重连
		if atomic.LoadInt32(&c.active) == 0 {
			return
		}

		log.Println("connection lost, reconnecting...")
		time.Sleep(3 * time.Second)
	}
}

func (c *Client) run() {
	defer c.conn.Close()

	go c.startHeartbeat()

	for {
		var msg WsMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			log.Println("read error:", err)
			return // 网络错误 → 触发重连
		}

		switch msg.Type {
		case "set_password":
			c.result <- msg.Data
			c.Close() // ✅ 主动关闭
			return
		}
	}
}

func (c *Client) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.conn.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(10*time.Second),
			); err != nil {
				log.Println("heartbeat failed:", err)
				return // ❗触发重连
			}
		}
	}
}
func (c *Client) Close() {
	if !atomic.CompareAndSwapInt32(&c.active, 1, 0) {
		return
	}
	c.stop()
	_ = c.conn.Close()
	close(c.result)
}

func WsHandleSetPassword() (string, error) {
	now := time.Now().Unix()
	signStr := fmt.Sprintf("%s:%d", devID, now)
	sign := util.HmacSign(signStr, ticket)
	url := fmt.Sprintf("wss://client.91demo.top/c/wsconn?devID=%s&devType=%s&timestamp=%d&signature=%s", devID, "domain", now, sign)
	client := NewClient(url)
	client.Start()
	result := <-client.result
	fmt.Println("result,", result)
	wsDevID := result["devID"].(string)
	password := result["password"].(string)
	if wsDevID != devID {
		return "", errors.New("devID不匹配")
	}

	wsReslut := fmt.Sprintf("%s,%s", devID, password)
	return wsReslut, nil
}
