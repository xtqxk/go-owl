package owl

import (
	"context"
	"log"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

type consulConfigure struct {
	keys               map[string]reflect.Value //full key path: attrs
	keyChangeCallbacks map[reflect.Value]struct {
		key       string
		callbacks []reflect.Value
	}
	cfg        interface{}
	consulAddr string
	baseKey    string
	ctx        context.Context
	client     *api.Client
	logger     *log.Logger
}

func New(ctx context.Context, cfg interface{}, consulAddr string, logger *log.Logger) (*consulConfigure, error) {
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
		return nil, err
	}
	cc.client = client
	if err := cc.initKeys(); err != nil {
		return nil, err
	}
	kv := client.KV()
	ks, _, err := kv.List(cc.baseKey, nil)
	if err != nil {
		return nil, err
	}
	if len(ks) > 0 {
		for _, k := range ks {
			if cValue, ok := cc.keys[k.Key]; ok {
				err := cc.updateKV(cValue, string(k.Value))
				if err != nil {
					logger.Fatalln(err.Error())
				}
			}
			logger.Printf("Key:%s,Value:%s\n", k.Key, string(k.Value))
		}
	}
	return cc, nil
}

func (c *consulConfigure) watchKey(key string, callbackFuns ...reflect.Value) error {
	pKey, err := c.getFullKeyPath(key)
	if err != nil {
		return err
	}
	params := map[string]interface{}{
		"type": "key",
		"key":  pKey,
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
			c.updateKV(cValue, string(v.Value))
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

func (c *consulConfigure) initKeys() error {
	rv := reflect.ValueOf(c.cfg)
	es := reflect.TypeOf(c.cfg).Elem()
	ev := rv.Elem()
	c.keys = make(map[string]reflect.Value, es.NumField())
	c.keyChangeCallbacks = make(map[reflect.Value]struct {
		key       string
		callbacks []reflect.Value
	})
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
			fullKey, err := c.getFullKeyPath(consulKey)
			if err != nil {
				return err
			}
			c.keys[fullKey] = fv

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
			c.keyChangeCallbacks[fv] = struct {
				key       string
				callbacks []reflect.Value
			}{
				key:       consulKey,
				callbacks: watchCallbackFuns,
			}
			defaultVal := f.Tag.Get("default")
			if len(defaultVal) > 0 {
				err := c.updateKV(fv, defaultVal)
				if err != nil {
					c.logger.Println(err.Error())
				}
			}
			if autoWatch {
				go c.watchKey(consulKey, watchCallbackFuns...)
			}
		}
	}
	return nil
}

func (c *consulConfigure) updateKV(cValue reflect.Value, val string) error {
	_, err := SetValue(cValue, val)
	if err != nil {
		c.logger.Printf("err:%s", err.Error())
		return err
	} else if _, ok := c.keyChangeCallbacks[cValue]; ok {
		item := c.keyChangeCallbacks[cValue]
		for _, function := range item.callbacks {
			args := []reflect.Value{reflect.ValueOf(item.key), reflect.ValueOf(val)}
			function.Call(args)
		}
	}
	return nil
}

func (c *consulConfigure) getFullKeyPath(key string) (string, error) {
	if c.baseKey == "" {
		return key, nil
	}
	return url.JoinPath(c.baseKey, key)
}
