package main

import (
	"LingFengShop/user_srv/proto"
	"context"
	"fmt"

	"google.golang.org/grpc"
)

var userClient proto.UserClient
var conn *grpc.ClientConn

func Init() {
	var err error
	conn, err = grpc.NewClient("127.0.0.1:5502", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	userClient = proto.NewUserClient(conn)
}

func TestGetUserList() {
	rsp, err := userClient.GetUserList(context.Background(), &proto.PageInfo{
		Pn:    1,
		PSize: 5,
	})
	if err != nil {
		panic(err)
	}
	for _, user := range rsp.Data {
		fmt.Println(user.Phone, user.NickName, user.PassWord)
		checkRsp, err := userClient.CheckPassWord(context.Background(), &proto.PasswordCheckInfo{
			Password:          "init password",
			EncryptedPassword: user.PassWord,
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(checkRsp.Success)
	}
}

func TestCreateUser() {
	for i := 0; i < 1; i++ {
		rsp, err := userClient.CreateUser(context.Background(), &proto.CreateUserInfo{
			NickName: fmt.Sprintf("bobby%d", i),
			Phone:    fmt.Sprintf("187822222%d2", i),
			PassWord: "admin123",
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(rsp.Id)
	}
}

func TestGetById() {
	rsp, err := userClient.GetUserById(context.Background(), &proto.IdRequest{Id: 13})
	fmt.Println(rsp)
	fmt.Println(err)
}

func main() {
	Init()
	// TestCreateUser()
	// TestGetUserList()
	TestGetById()
	conn.Close()
}
