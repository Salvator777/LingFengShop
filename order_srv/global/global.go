package global

import (
	"LingFengShop/order_srv/config"
	"LingFengShop/order_srv/proto"

	"gorm.io/gorm"
)

var (
	DB           *gorm.DB
	ServerConfig *config.ServerConfig = new(config.ServerConfig)
	NacosConfig  *config.NacosConfig  = &config.NacosConfig{}

	GoodsSrvClient     proto.GoodsClient
	InventorySrvClient proto.InventoryClient
)
