package global

import (
	"LingFengShop/goods_srv/config"

	"github.com/olivere/elastic/v7"
	"gorm.io/gorm"
)

var (
	DB *gorm.DB

	ServerConfig *config.ServerConfig = new(config.ServerConfig)

	NacosConfig *config.NacosConfig = &config.NacosConfig{}

	EsClient *elastic.Client
)
