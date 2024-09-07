package main

import (
	"context"
	"fmt"
	"log"

	"github.com/olivere/elastic/v7"
)

func main() {
	client, err := elastic.NewClient(elastic.SetSniff(false))
	if err != nil {
		log.Fatalln(err)
	}
	q := elastic.NewMatchQuery("address", "street")
	res, err := client.Search().Index("user").Query(q).Do(context.Background())
	if err != nil {
		log.Fatalln(err)
	}
	total := res.Hits.TotalHits.Value
	fmt.Println("搜索结果数量：", total)

	for _, v := range res.Hits.Hits {
		if jsonDate, err := v.Source.MarshalJSON(); err == nil {
			fmt.Println(string(jsonDate))
		} else {
			panic(err)
		}
	}
}
