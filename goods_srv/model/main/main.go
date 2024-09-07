package main

// 这个文件专门用来同步数据库表

import (
	"LingFengShop/goods_srv/global"
	"LingFengShop/goods_srv/model"
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/olivere/elastic/v7"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func main() {
	// dsn := "root:243326@tcp(127.0.0.1:3306)/LFshop_goods_srv?charset=utf8mb4&parseTime=True&loc=Local"

	// // 设置全局的logger，这个logger在我们执行每个sql语句的时候会打印每一行sql
	// newLogger := logger.New(
	// 	log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
	// 	logger.Config{
	// 		SlowThreshold: time.Second, // 慢 SQL 阈值
	// 		LogLevel:      logger.Info, // Log level
	// 		Colorful:      true,        // 禁用彩色打印
	// 	},
	// )

	// // 全局模式
	// db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
	// 	NamingStrategy: schema.NamingStrategy{
	// 		// 让创建表名称时，不要自动加s
	// 		SingularTable: true,
	// 	},
	// 	Logger: newLogger,
	// })
	// if err != nil {
	// 	panic(err)
	// }

	// err = db.AutoMigrate(&model.Category{}, &model.Brands{}, &model.GoodsCategoryBrand{},
	// 	&model.Banner{}, &model.Goods{})
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	Mysql2Es()
}

func Mysql2Es() {
	dsn := "root:243326@tcp(127.0.0.1:3306)/LFshop_goods_srv?charset=utf8mb4&parseTime=True&loc=Local"

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold: time.Second, // 慢 SQL 阈值
			LogLevel:      logger.Info, // Log level
			Colorful:      true,        // 禁用彩色打印
		},
	)

	// 全局模式
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		Logger: newLogger,
	})
	if err != nil {
		panic(err)
	}

	host := "http://10.237.56.85:9200"
	logger := log.New(os.Stdout, "LFshop", log.LstdFlags)
	global.EsClient, err = elastic.NewClient(elastic.SetURL(host), elastic.SetSniff(false),
		elastic.SetTraceLog(logger))
	if err != nil {
		panic(err)
	}

	var goods []model.Goods
	db.Find(&goods)
	for _, g := range goods {
		esModel := model.EsGoods{
			ID:          g.ID,
			CategoryID:  g.CategoryID,
			BrandsID:    g.BrandsID,
			OnSale:      g.OnSale,
			ShipFree:    g.ShipFree,
			IsNew:       g.IsNew,
			IsHot:       g.IsHot,
			Name:        g.Name,
			ClickNum:    g.ClickNum,
			SoldNum:     g.SoldNum,
			FavNum:      g.FavNum,
			MarketPrice: g.MarketPrice,
			GoodsBrief:  g.GoodsBrief,
			ShopPrice:   g.ShopPrice,
		}

		_, err = global.EsClient.Index().Index(esModel.GetIndexName()).BodyJson(esModel).Id(strconv.Itoa(int(g.ID))).Do(context.Background())
		if err != nil {
			panic(err)
		}
		//强调一下 一定要将docker启动es的java_ops的内存设置大一些 否则运行过程中会出现 bad request错误
	}
}