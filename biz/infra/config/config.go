package config

import (
	"os"
	"sync"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

var (
	config *Config
	once   sync.Once
)

type Auth struct {
	SecretKey    string
	PublicKey    string
	AccessExpire int64
}

type Mongo struct {
	URL string
	DB  string
}

type Deyu struct {
	Name    string
	APIKey  string
	BaseURL string
	BotID   string `json:",omitempty"`
}

type SMSConfig struct {
	Account string
	Token   string
}

type Config struct {
	service.ServiceConf
	ListenOn string
	State    string
	Auth     Auth
	SMS      *SMSConfig
	Models   map[string]*Deyu
	Cache    cache.CacheConf
	Redis    *redis.RedisConf
	Mongo    Mongo
}

func NewConfig() (*Config, error) {
	once.Do(func() {
		c := new(Config)
		path := os.Getenv("CONFIG_PATH")
		if path == "" {
			path = "etc/config.yaml"
		}
		err := conf.Load(path, c)
		if err != nil {
			panic(err)
		}
		err = c.SetUp()
		if err != nil {
			panic(err)
		}
		config = c
	})

	return config, nil
}

func GetConfig() *Config {
	once.Do(func() {
		_, _ = NewConfig()
	})
	return config
}
