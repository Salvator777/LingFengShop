package handler

import (
	"LingFengShop/goods_srv/global"
	"LingFengShop/goods_srv/model"
	"LingFengShop/goods_srv/proto"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// 品牌
func (s *GoodsServer) BrandList(ctx context.Context, req *proto.BrandFilterRequest) (*proto.BrandListResponse, error) {
	var brands []model.Brands
	result := global.DB.Scopes(Paginate(int(req.Pages), int(req.PagePerNums))).Find(&brands)
	if result.Error != nil {
		return nil, result.Error
	}

	brandListResponse := &proto.BrandListResponse{}
	// 查询表的所有记录总数
	var total int64
	global.DB.Model(&model.Brands{}).Count(&total)
	brandListResponse.Total = int32(total)

	for _, brand := range brands {
		brandListResponse.Data = append(brandListResponse.Data, &proto.BrandInfoResponse{
			Id:   brand.ID,
			Name: brand.Name,
			Logo: brand.Logo,
		})
	}
	return brandListResponse, nil
}

func (s *GoodsServer) CreateBrand(ctx context.Context, req *proto.BrandRequest) (*proto.BrandInfoResponse, error) {
	//新建品牌
	if result := global.DB.Where("name=?", req.Name).First(&model.Brands{}); result.RowsAffected == 1 {
		return nil, status.Errorf(codes.InvalidArgument, "品牌已存在")
	}

	brand := &model.Brands{
		Name: req.Name,
		Logo: req.Logo,
	}
	global.DB.Save(brand)
	return &proto.BrandInfoResponse{Id: brand.ID}, nil
}

func (s *GoodsServer) DeleteBrand(ctx context.Context, req *proto.BrandRequest) (*emptypb.Empty, error) {
	if result := global.DB.Delete(&model.Brands{}, req.Id); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "品牌不存在")
	}
	return &emptypb.Empty{}, nil
}

func (s *GoodsServer) UpdateBrand(ctx context.Context, req *proto.BrandRequest) (*emptypb.Empty, error) {
	// 更新就是先找到对象，再save
	brands := model.Brands{}
	if result := global.DB.First(&brands); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "品牌不存在")
	}

	if req.Name != "" {
		brands.Name = req.Name
	}
	if req.Logo != "" {
		brands.Logo = req.Logo
	}

	global.DB.Save(&brands)

	return &emptypb.Empty{}, nil
}
