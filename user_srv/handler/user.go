package handler

// 这个文件来实现proto定义的具体方法

import (
	"LingFengShop/user_srv/global"
	"LingFengShop/user_srv/model"
	"LingFengShop/user_srv/proto"
	"context"
	"crypto/sha512"
	"fmt"
	"strings"
	"time"

	"github.com/anaskhan96/go-password-encoder"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// 用这个对象，封装和user相关的grpc方法
type UserServer struct {
	proto.UnimplementedUserServer
}

// mustEmbedUnimplementedUserServer implements proto.UserServer.
func (s *UserServer) mustEmbedUnimplementedUserServer() {
	panic("unimplemented")
}

// 把user对象转成UserInfoResponse
func ModelToRsponse(user model.User) proto.UserInfoResponse {
	userInfoRsp := proto.UserInfoResponse{
		Id:       int32(user.ID),
		PassWord: user.Password,
		NickName: user.NickName,
		Gender:   user.Gender,
		Role:     int32(user.Role),
		Phone:    user.Phone,
	}
	// 在grpc的message中字段有默认值，不能随便赋值nil进去，容易出错
	// 因此对于有可能是Nil的字段，要判断不为空再赋值
	// user.BirthDay没有默认值，有可能为nil，这种情况单独赋值
	if user.Birthday != nil {
		userInfoRsp.BirthDay = uint64(user.Birthday.Unix())
	}
	return userInfoRsp
}

// gorm官方提供的分页函数，进行了一定修改，实现优雅的分页
func Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page == 0 {
			page = 1
		}

		switch {
		case pageSize > 100:
			pageSize = 100
		case pageSize <= 0:
			pageSize = 10
		}

		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize)
	}
}

func (s *UserServer) GetUserList(ctx context.Context, req *proto.PageInfo) (*proto.UserListResponse, error) {
	fmt.Println(1111111)
	//获取用户列表
	var users []model.User
	result := global.DB.Find(&users)
	if result.Error != nil {
		return nil, result.Error
	}
	rsp := &proto.UserListResponse{}
	rsp.Total = int32(result.RowsAffected)

	// 分页查询的用法，Psize是每页的数据量，Pn是想查哪页的数据
	global.DB.Scopes(Paginate(int(req.Pn), int(req.PSize))).Find(&users)

	for _, user := range users {
		userInfoRsp := ModelToRsponse(user)
		rsp.Data = append(rsp.Data, &userInfoRsp)
	}
	return rsp, nil
}

// 通过手机号码查询用户
func (s *UserServer) GetUserByPhone(ctx context.Context, req *proto.PhoneRequest) (*proto.UserInfoResponse, error) {
	var user model.User
	result := global.DB.Where(&model.User{Phone: req.Phone}).First(&user)
	if result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "用户不存在")
	}
	if result.Error != nil {
		return nil, result.Error
	}

	userInfoRsp := ModelToRsponse(user)
	return &userInfoRsp, nil
}

// 通过id查询用户
func (s *UserServer) GetUserById(ctx context.Context, req *proto.IdRequest) (*proto.UserInfoResponse, error) {
	var user model.User
	result := global.DB.First(&user, req.Id)
	if result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "用户不存在")
	}
	if result.Error != nil {
		return nil, result.Error
	}

	userInfoRsp := ModelToRsponse(user)
	return &userInfoRsp, nil
}

// 新建用户，只需要用户名，手机号，密码
func (s *UserServer) CreateUser(ctx context.Context, req *proto.CreateUserInfo) (*proto.UserInfoResponse, error) {
	var user model.User
	result := global.DB.Where(&model.User{Phone: req.Phone}).First(&user)
	if result.RowsAffected == 1 {
		// 使用grpc包专门的出列错误的方式
		return nil, status.Errorf(codes.AlreadyExists, "用户已存在")
	}

	user.Phone = req.Phone
	user.NickName = req.NickName

	//密码加密
	options := &password.Options{16, 100, 32, sha512.New}
	salt, encodedPwd := password.Encode(req.PassWord, options)
	user.Password = fmt.Sprintf("$pbkdf2-sha512$%s$%s", salt, encodedPwd)

	result = global.DB.Create(&user)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, result.Error.Error())
	}

	userInfoRsp := ModelToRsponse(user)
	return &userInfoRsp, nil
}

// 个人中心更新用户
func (s *UserServer) UpdateUser(ctx context.Context, req *proto.UpdateUserInfo) (*empty.Empty, error) {
	var user model.User
	result := global.DB.First(&user, req.Id)
	if result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "用户不存在")
	}

	birthDay := time.Unix(int64(req.BirthDay), 0)
	user.NickName = req.NickName
	user.Birthday = &birthDay
	user.Gender = req.Gender

	result = global.DB.Save(&user)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, result.Error.Error())
	}
	// 在gRPC中，empty.Empty用于不需要返回任何数据的方法，表示操作成功
	return &empty.Empty{}, nil
}

func (s *UserServer) CheckPassWord(ctx context.Context, req *proto.PasswordCheckInfo) (*proto.CheckResponse, error) {
	//校验密码
	options := &password.Options{16, 100, 32, sha512.New}
	passwordInfo := strings.Split(req.EncryptedPassword, "$")
	check := password.Verify(req.Password, passwordInfo[2], passwordInfo[3], options)
	return &proto.CheckResponse{Success: check}, nil
}
