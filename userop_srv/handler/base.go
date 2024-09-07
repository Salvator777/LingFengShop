package handler

import (
	"LingFengShop/userop_srv/proto"

	"gorm.io/gorm"
)

// 把关于三个表的操作封装成一个UserOpServer
// 三个go文件的方法都写成UserOpServer的方法即可
type UserOpServer struct {
	proto.UnimplementedAddressServer
	proto.UnimplementedUserFavServer
	proto.UnimplementedMessageServer
}

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
