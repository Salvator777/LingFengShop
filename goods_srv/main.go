package main

import (
	"LingFengShop/goods_srv/global"
	"LingFengShop/goods_srv/handler"
	"LingFengShop/goods_srv/initialize"
	"LingFengShop/goods_srv/proto"
	"LingFengShop/goods_srv/utils"
	"LingFengShop/goods_srv/utils/register/consul"
	"flag"
	"fmt"
	"net"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// 启动grpc服务器
func main() {
	IP := flag.String("ip", "0.0.0.0", "ip地址")
	Port := flag.Int("port", 0, "端口号")
	flag.Parse()
	// 命令行没传port就随机
	if *Port == 0 {
		*Port, _ = utils.GetFreePort()
	}

	// 初始化
	initialize.InitLogger()
	initialize.InitConfig()
	initialize.InitDB()
	initialize.InitEs()

	zap.S().Info("ip：", *IP)
	zap.S().Info("port：", *Port)

	server := grpc.NewServer()
	// 生成代码后，必须加一个proto.UnimplementedUserServer字段
	proto.RegisterGoodsServer(server, &handler.GoodsServer{})
	listner, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *IP, *Port))
	if err != nil {
		panic("failed to listen:" + err.Error())
	}

	// 添加健康监测
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	// 注册服务
	register_client := consul.NewRegistryClient(global.ServerConfig.ConsulInfo.Host, global.ServerConfig.ConsulInfo.Port)
	serviceId := uuid.NewString()
	err = register_client.Register(global.ServerConfig.Host, *Port, global.ServerConfig.Name, global.ServerConfig.Tags, serviceId)
	if err != nil {
		zap.S().Panic("goods-srv服务注册失败:", err.Error())
	}

	err = server.Serve(listner)
	if err != nil {
		panic("failed to start grpc:" + err.Error())
	}
}
