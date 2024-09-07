package main

import (
	"LingFengShop/user_srv/global"
	"LingFengShop/user_srv/handler"
	"LingFengShop/user_srv/initialize"
	"LingFengShop/user_srv/proto"
	"LingFengShop/user_srv/utils"
	"flag"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
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

	zap.S().Info("ip：", *IP)
	zap.S().Info("port：", *Port)

	server := grpc.NewServer()
	// 生成代码后，必须加一个proto.UnimplementedUserServer字段
	proto.RegisterUserServer(server, &handler.UserServer{})
	listner, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *IP, *Port))
	if err != nil {
		panic("failed to listen:" + err.Error())
	}

	// 添加健康监测
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())

	// 服务注册
	// 拿到consul客户端
	cfg := api.DefaultConfig()
	cfg.Address = fmt.Sprintf("%s:%d", global.ServerConfig.ConsulInfo.Host,
		global.ServerConfig.ConsulInfo.Port)
	client, err := api.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	//生成对应的检查对象
	// 注意：GRPC的ip不能写本地回环地址，要写本机的真实ip地址，下面的registration.Address也一样
	check := &api.AgentServiceCheck{
		GRPC:                           fmt.Sprintf("10.237.56.85:%d", *Port),
		Timeout:                        "5s",
		Interval:                       "5s",
		DeregisterCriticalServiceAfter: "15s",
	}

	//生成注册对象
	registration := new(api.AgentServiceRegistration)
	registration.Name = global.ServerConfig.Name
	registration.ID = uuid.NewString()
	registration.Port = *Port
	registration.Tags = []string{"LFshop", "sp", "user", "srv"}
	registration.Address = "10.237.56.85"
	registration.Check = check
	//1. 如何启动两个服务
	//2. 即使我能够通过终端启动两个服务，但是注册到consul中的时候也会被覆盖
	err = client.Agent().ServiceRegister(registration)
	if err != nil {
		panic(err)
	}

	err = server.Serve(listner)
	if err != nil {
		panic("failed to start grpc:" + err.Error())
	}
}
