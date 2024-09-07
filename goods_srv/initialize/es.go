package initialize

import (
	"LingFengShop/goods_srv/global"
	"LingFengShop/goods_srv/model"
	"context"
	"fmt"
	"log"

	"os"

	"github.com/olivere/elastic/v7"
)

func InitEs() {
	//初始化连接
	host := fmt.Sprintf("http://%s:%d", global.ServerConfig.EsInfo.Host, global.ServerConfig.EsInfo.Port)
	logger := log.New(os.Stdout, "LFshop", log.LstdFlags)
	var err error
	global.EsClient, err = elastic.NewClient(elastic.SetURL(host), elastic.SetSniff(false),
		elastic.SetTraceLog(logger))
	if err != nil {
		panic(err)
	}

	//新建mapping和index
	exists, err := global.EsClient.IndexExists(model.EsGoods{}.GetIndexName()).Do(context.Background())
	if err != nil {
		panic(err)
	}
	if !exists {
		_, err = global.EsClient.CreateIndex(model.EsGoods{}.GetIndexName()).BodyString(model.EsGoods{}.GetMapping()).Do(context.Background())
		if err != nil {
			panic(err)
		}
	}
}
