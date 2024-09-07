package initialize

import (
	"LingFengShop/userop_srv/global"
	"encoding/json"
	"fmt"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func GetEnvInfo(env string) bool {
	viper.AutomaticEnv()
	return viper.GetBool(env)
}

func InitConfig() {
	debug := GetEnvInfo("LFSHOP_DEBUG")
	configFilePrefix := "config"
	configFileName := fmt.Sprintf("userop_srv/%s-pro.yaml", configFilePrefix)
	if debug {
		configFileName = fmt.Sprintf("userop_srv/%s-debug.yaml", configFilePrefix)
	}

	v := viper.New()
	v.SetConfigFile(configFileName)
	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}
	if err := v.Unmarshal(global.NacosConfig); err != nil {
		panic(err)
	}
	zap.S().Infof("配置信息: %v", global.NacosConfig)

	//从nacos中读取配置信息
	// sc是nacos服务的ip和port
	sc := []constant.ServerConfig{
		{
			IpAddr: global.NacosConfig.Host,
			Port:   global.NacosConfig.Port,
		},
	}

	// cc是本地服务作为nacos客户端的配置
	cc := constant.ClientConfig{
		NamespaceId:         global.NacosConfig.Namespace, // 如果需要支持多namespace，我们可以场景多个client,它们有不同的NamespaceId
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		// nacos会在本地放一些拉取下来的配置缓存，以及放一些日志
		LogDir:   "tmp/nacos/log",
		CacheDir: "tmp/nacos/cache",
		LogLevel: "debug",
	}

	// 拿到客户端
	configClient, err := clients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": sc,
		"clientConfig":  cc,
	})
	if err != nil {
		panic(err)
	}

	// 通过nacos拿到配置，配置是json格式，需要解析到global的变量中
	content, err := configClient.GetConfig(vo.ConfigParam{
		DataId: global.NacosConfig.DataId,
		Group:  global.NacosConfig.Group})
	if err != nil {
		panic(err)
	}
	//想要将一个json字符串转换成struct，需要去设置这个struct的tag
	err = json.Unmarshal([]byte(content), global.ServerConfig)
	if err != nil {
		zap.S().Fatalf("读取nacos配置失败： %s", err.Error())
	}
	fmt.Println(global.ServerConfig)
}
