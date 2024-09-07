package main

import (
	"LingFengShop/goods_srv/proto"
	"context"
	"fmt"

	"google.golang.org/grpc"
)

var brandClient proto.GoodsClient
var conn *grpc.ClientConn

func Init() {
	var err error
	conn, err = grpc.NewClient("127.0.0.1:5809", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	brandClient = proto.NewGoodsClient(conn)
}

func TestGetBrandList() {
	rsp, err := brandClient.BrandList(context.Background(), &proto.BrandFilterRequest{
		Pages:       0,
		PagePerNums: 0,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(rsp.Total)
	for _, brand := range rsp.Data {
		fmt.Println(brand.Name)
	}
}

func main() {
	Init()
	TestGetBrandList()
	conn.Close()
}
