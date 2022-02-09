package main

import (
	"encoding/json"
	"log"

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
	owl.New(cfg, "consul-server-addr:8500")
	select {}
}
