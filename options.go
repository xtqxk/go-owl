package owl

import (
	"context"
	"log"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

type consulConfigure struct {
	keys       map[string]reflect.Value
	cfg        interface{}
	consulAddr string
	baseKey    string
	ctx        context.Context
	client     *api.Client
	logger     *log.Logger
}

func New(ctx context.Context, cfg interface{}, consulAddr string, logger *log.Logger) *consulConfigure {
	if logger == nil {
		logger = log.Default()
	}
	cc := &consulConfigure{
		cfg:        cfg,
		consulAddr: consulAddr,
		ctx:        ctx,
		logger:     logger,
	}
	consulCfg := api.DefaultConfig()
	consulCfg.Address = consulAddr
	client, err := api.NewClient(consulCfg)
	if err != nil {
		panic(err)
	}
	cc.client = client
	cc.initKeys()
	kv := client.KV()
	ks, _, err := kv.List(cc.baseKey, nil)
	if err != nil {
		panic(err.Error())
	}
	if len(ks) > 0 {
		for _, k := range ks {
			if cValue, ok := cc.keys[k.Key]; ok {
				_, err := SetValue(cValue, string(k.Value))
				if err != nil {
					logger.Fatalln(err.Error())
				}
			}
			logger.Printf("Key:%s,Value:%s\n", k.Key, string(k.Value))
		}
	}
	return cc
}

func (c *consulConfigure) watchKey(key string, callbackFuns ...reflect.Value) error {
	params := map[string]interface{}{
		"type": "key",
		"key":  c.getFullKeyPath(key),
	}
	plan, err := watch.Parse(params)
	if err != nil {
		return err
	}
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return // ignore
		}
		v, ok := raw.(*api.KVPair)
		if !ok || v == nil {
			return // ignore
		}
		if cValue, ok := c.keys[v.Key]; ok {
			_, err := SetValue(cValue, string(v.Value))
			if err != nil {
				c.logger.Printf("err:%s", err.Error())
			} else if len(callbackFuns) > 0 {
				for _, function := range callbackFuns {
					args := []reflect.Value{reflect.ValueOf(key), reflect.ValueOf(string(v.Value))}
					function.Call(args)
				}
			}
		}
		c.logger.Printf(">>>%s:%s", string(v.Key), string(v.Value))
	}

LOOP:
	for {
		watchChan := make(chan error)
		go func() {
			defer close(watchChan)
			if err := plan.RunWithClientAndLogger(c.client, c.logger); err != nil {
				c.logger.Println("--->", err.Error())
				watchChan <- err
				return
			}
			watchChan <- nil
		}()
		select {
		case <-c.ctx.Done():
			c.logger.Printf("%s->watcher close", key)
			time.Sleep(2 * time.Second)
			if !plan.IsStopped() {
				plan.Stop()
			}
			break LOOP
		case <-watchChan:
		}
	}
	return nil
}

func (c *consulConfigure) initKeys() {
	t := reflect.TypeOf(c.cfg)
	rv := reflect.ValueOf(c.cfg)
	es := t.Elem()
	ev := rv.Elem()
	c.keys = make(map[string]reflect.Value, es.NumField())
	for i := 0; i < es.NumField(); i++ {
		f := es.Field(i)
		fv := ev.FieldByName(f.Name)
		if !fv.CanSet() {
			if f.Name == "_baseKey" {
				if theDefaultValue := f.Tag.Get("default"); theDefaultValue != "" {
					c.baseKey = theDefaultValue
				} else {
					continue
				}
			}
		} else {
			var consulTag string
			if consulTag = f.Tag.Get("consul"); consulTag == "" {
				continue
			}
			var consulKey, consulValStr string
			tagParts := strings.Split(consulTag, ":")
			consulKey = tagParts[0]
			autoWatch := false
			if len(tagParts) > 1 {
				consulValStr = tagParts[1]
			}
			fullKey := c.getFullKeyPath(consulKey)
			c.keys[fullKey] = fv
			defaultVal := f.Tag.Get("default")
			if len(defaultVal) > 0 {
				_, err := SetValue(fv, defaultVal)
				if err != nil {
					c.logger.Println(err.Error())
				}
			}

			watchCallbackFuns := []reflect.Value{}
			if len(consulValStr) > 0 {
				funcs := strings.Split(consulValStr, ",")
				for _, val := range funcs {
					if val == "*" {
						autoWatch = true
					} else if v := rv.MethodByName(val); v.IsValid() {
						watchCallbackFuns = append(watchCallbackFuns, v)
						autoWatch = true
					}
				}
			}
			if autoWatch {
				go c.watchKey(consulKey, watchCallbackFuns...)
			}
		}
	}
}

func (c *consulConfigure) getFullKeyPath(key string) string {
	if c.baseKey == "" {
		return key
	}
	return path.Join(c.baseKey, key)
}
