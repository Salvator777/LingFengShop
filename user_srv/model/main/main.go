package main

// 这个文件专门用来同步数据库表

import (
	"LingFengShop/user_srv/model"
	"crypto/md5"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/anaskhan96/go-password-encoder"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// 生成16位的md5密文
func genMd5(code string) string {
	Md5 := md5.New()
	_, _ = io.WriteString(Md5, code)
	return hex.EncodeToString(Md5.Sum(nil))
}

func main() {
	dsn := "root:243326@tcp(127.0.0.1:3306)/LFshop_user_srv?charset=utf8mb4&parseTime=True&loc=Local"

	// 设置全局的logger，这个logger在我们执行每个sql语句的时候会打印每一行sql
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
			// 让创建表名称时，不要自动加s
			SingularTable: true,
		},
		Logger: newLogger,
	})
	if err != nil {
		panic(err)
	}

	err = db.AutoMigrate(&model.User{})
	if err != nil {
		log.Fatalln(err)
	}

	// 配置信息，生成16位盐值，迭代100次，key长度32，采用sha512算法
	options := &password.Options{16, 100, 32, sha512.New}
	salt, encodedPwd := password.Encode("init password", options)

	// 数据表里的password存这个，长度96，不要超过数据库password规定的100
	newPassword := fmt.Sprintf("$pbkdf2-sha512$%s$%s", salt, encodedPwd)

	// 生成10个用户
	// for i := 0; i < 10; i++ {
	// 	user := model.User{
	// 		NickName: fmt.Sprintf("bobby%d", i),
	// 		Phone:    fmt.Sprintf("1878222222%d", i),
	// 		Password: newPassword,
	// 	}
	// 	db.Save(&user)
	// }

	passwordInfo := strings.Split(newPassword, "$")
	fmt.Println(len(newPassword)) //
	fmt.Println(passwordInfo)

	// 验证密码是否正确
	check := password.Verify("init password", salt, encodedPwd, options)
	fmt.Println(check) // true
}
