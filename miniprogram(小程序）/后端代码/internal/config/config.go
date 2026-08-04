package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Port int    `mapstructure:"port"`
		Mode string `mapstructure:"mode"`
	} `mapstructure:"server"`
	MySQL struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Database string `mapstructure:"database"`
	} `mapstructure:"mysql"`
	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`
	JWT struct {
		Secret      string `mapstructure:"secret"`
		ExpireHours int    `mapstructure:"expire_hours"`
	} `mapstructure:"jwt"`
	WeChat struct {
		AppID  string `mapstructure:"appid"`
		Secret string `mapstructure:"secret"`
	} `mapstructure:"wechat"`
	AllowDevBackdoor  bool   `mapstructure:"allow_dev_backdoor"`
	TeacherInviteCode string `mapstructure:"teacher_invite_code"`
	CORS              struct {
		AllowOrigins []string `mapstructure:"allow_origins"`
	} `mapstructure:"cors"`
	RateLimit struct {
		LoginPerMinute  int `mapstructure:"login_per_minute"`
		SubmitPerMinute int `mapstructure:"submit_per_minute"`
	} `mapstructure:"rate_limit"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) MySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.MySQL.User, c.MySQL.Password, c.MySQL.Host, c.MySQL.Port, c.MySQL.Database)
}
