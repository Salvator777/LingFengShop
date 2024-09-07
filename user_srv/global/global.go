package global

import (
	"LingFengShop/user_srv/config"

	"gorm.io/gorm"
)

var (
	DB *gorm.DB

	ServerConfig *config.ServerConfig = new(config.ServerConfig)

	NacosConfig *config.NacosConfig = &config.NacosConfig{}
)
