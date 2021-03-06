package benchmark

import (
	"encoding/json"
	"errors"
	"time"
	"github.com/xuperchain/xuperbench/config"
	"github.com/xuperchain/xuperbench/log"
	"github.com/gomodule/redigo/redis"
)

// Subscribe sub the chan and wait for BenchMsg then do bench.
func Subscribe(conf *config.Config) {
	log.DEBUG.Println(conf.Broker)
	c, err := redis.Dial("tcp", conf.Broker)
	if err != nil {
		log.ERROR.Println(err)
		return
	}
	defer c.Close()

	psc := redis.PubSubConn{Conn: c}
	psc.Subscribe(conf.PubSubChan)

loop:
	for {
		log.INFO.Printf("wait message to benchmark......")

		switch v := psc.Receive().(type) {
		case redis.Message:
			log.INFO.Printf("received message from <%s>: %s", v.Channel, v.Data)

			bench := func(data []byte) {
				conf := &config.Config{}
				err := json.Unmarshal(data, &conf)
				if err != nil {
					log.ERROR.Println("sub err: ", err)
					return
				}

				BenchRun(conf)
			}

			bench(v.Data)
			break loop
		case redis.Subscription:
			log.DEBUG.Printf("receive redis.Subscription form <%s>, Kind: %s, Count: %d", v.Channel, v.Kind, v.Count)
		case error:
			log.ERROR.Printf("encounted error <%s> while receiving msg", v)
		default:
			log.INFO.Printf("receive no msg")
		}

		time.Sleep(2 * time.Second)
	}
}

// Publish pub the BenchMsg to the chan.
func Publish(conf *config.Config) {
	c, err := redis.Dial("tcp", conf.Broker)
	if err != nil {
		log.ERROR.Println(err)
		return
	}
	defer c.Close()

	msg, err := json.Marshal(*conf)
	if err != nil {
		log.ERROR.Println(err)
		return
	}
	log.INFO.Printf("bench config msg: %v\n", string(msg))

	_, err = c.Do("PUBLISH", conf.PubSubChan, string(msg))
	if err != nil {
		log.ERROR.Println("pub err: ", err)
		return
	}
	// BackendProf(conf)
}

func Wait(conf *config.Config, postfix string) error {
	c, err := redis.Dial("tcp", conf.Broker)
	retryout := 120
	if err != nil {
		log.ERROR.Println(err)
		return err
	}
	defer c.Close()
	key := conf.PubSubChan + "_" + postfix
	target, _ := redis.Int(c.Do("GET", conf.PubSubChan + "_worker"))

	for i:=0; i<retryout; i++ {
		val, _ := redis.Int(c.Do("GET", key))
		if val == target {
			return nil
		}
		log.DEBUG.Printf("waiting for other workers ...")
		time.Sleep(1 * time.Second)
	}
	return errors.New("wait timeout")
}

func Set(conf *config.Config, postfix string) error {
	c, err := redis.Dial("tcp", conf.Broker)
	if err != nil {
		log.ERROR.Println(err)
		return err
	}
	defer c.Close()
	key := conf.PubSubChan + "_" + postfix
	_, err = c.Do("INCR", key)
	return err
}
