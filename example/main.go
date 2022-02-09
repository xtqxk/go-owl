package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xtqxk/go-owl"
)

type demoCfg struct {
	_baseKey string  `default:"DemoProject"`
	Token    bool    `consul:"token" default:"true" json:"token"`
	APIURL   string  `consul:"api-url" default:"http://www.demo.com" json:"api-url"`
	APIPort  *int    `consul:"api-port:APIPortUpdateHandler" json:"api-port"`
	Redis    *string `consul:"user-redis:RedisUpdateHandler" json:"user-redis"`
}

func (d *demoCfg) RedisUpdateHandler(key, val string) {
	log.Printf("RedisUpdateHandler:key:%s,val:%s", key, val)
	jsonBytes, _ := json.Marshal(d)
	log.Println(string(jsonBytes))
}

func (d *demoCfg) APIPortUpdateHandler(key, val string) {
	log.Printf("APIPortUpdateHandler:key:%s,val:%s", key, val)
	jsonBytes, _ := json.Marshal(d)
	log.Println(string(jsonBytes))
}

func main() {
	cfg := new(demoCfg)
	ctx, cancel := context.WithCancel(context.Background())
	owl.New(ctx, cfg, "consul-server-addr:8500")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Println("cancel")
	cancel()
	<-ctx.Done()
	time.Sleep(1 * time.Second)
	log.Println("bye!")
}
