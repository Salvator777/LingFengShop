package handler

import (
	"LingFengShop/goods_srv/global"
	"LingFengShop/goods_srv/model"
	"LingFengShop/goods_srv/proto"
	"context"
	"encoding/json"
	"fmt"

	"github.com/olivere/elastic/v7"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

type GoodsServer struct {
	// proto的所有方法，默认必须都实现才能启动服务
	// 但是可以在server里加一个UnimplementedGoodsServer，这个对象绑定了所有方法，返回没实现的错误
	// 加了这个，就可以不实现方法也能启动服务
	proto.UnimplementedGoodsServer
}

// mustEmbedUnimplementedUserServer implements proto.UserServer.
func (s *GoodsServer) mustEmbedUnimplementedUserServer() {
	panic("unimplemented")
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

// 把goods对象转成info，因为字段太多，单独提出来
func ModelToResponse(goods model.Goods) proto.GoodsInfoResponse {
	return proto.GoodsInfoResponse{
		Id:              goods.ID,
		CategoryId:      goods.CategoryID,
		Name:            goods.Name,
		GoodsSn:         goods.GoodsSn,
		ClickNum:        goods.ClickNum,
		SoldNum:         goods.SoldNum,
		FavNum:          goods.FavNum,
		MarketPrice:     goods.MarketPrice,
		ShopPrice:       goods.ShopPrice,
		GoodsBrief:      goods.GoodsBrief,
		ShipFree:        goods.ShipFree,
		GoodsFrontImage: goods.GoodsFrontImage,
		IsNew:           goods.IsNew,
		IsHot:           goods.IsHot,
		OnSale:          goods.OnSale,
		DescImages:      goods.DescImages,
		Images:          goods.Images,
		Category: &proto.CategoryBriefInfoResponse{
			Id:   goods.Category.ID,
			Name: goods.Category.Name,
		},
		Brand: &proto.BrandInfoResponse{
			Id:   goods.Brands.ID,
			Name: goods.Brands.Name,
			Logo: goods.Brands.Logo,
		},
	}
}

// 非常重要的方法，根据筛选条件查商品
// 老的方法，用的mysql查询
func (s *GoodsServer) GoodsList2(ctx context.Context, req *proto.GoodsFilterRequest) (*proto.GoodsListResponse, error) {
	//关键词搜索、查询新品、查询热门商品、通过价格区间筛选， 通过商品分类筛选
	goodsListResponse := &proto.GoodsListResponse{}
	var goods []model.Goods

	// 这里想连续查询，又不能影响全局的global，需要定义临时商品用的query

	// 使用es的bool查询
	query := global.DB.Model(model.Goods{})
	if req.KeyWords != "" {
		query = query.Where("name like ?", "%"+req.KeyWords+"%")
	}
	if req.IsHot {
		query = query.Where("is_hot = true")
	}
	if req.IsNew { // 上下两种查询都可以
		query = query.Where(model.Goods{IsNew: true})
	}
	if req.PriceMin > 0 {
		query = query.Where("shop_price >= ?", req.PriceMin)
	}
	if req.PriceMax > 0 {
		query = query.Where("shop_price <= ?", req.PriceMax)
	}
	if req.Brand > 0 {
		query = query.Where("brands_id = ?", req.Brand)
	}

	var sql string
	// 通过category去查询，要注意类别层级一共有三层
	// 要求查的可能是任意一层，需要把所有孩子都查出来
	if req.TopCategory > 0 {
		var category model.Category
		if result := global.DB.First(&category, req.TopCategory); result.RowsAffected == 0 {
			return nil, status.Errorf(codes.NotFound, "商品分类不存在")
		}
		if category.Level == 1 {
			sql = fmt.Sprintf("SELECT id FROM category WHERE category.parent_category_id in(SELECT id FROM category WHERE category.parent_category_id = %d)", req.TopCategory)
		} else if category.Level == 2 {
			sql = fmt.Sprintf("SELECT id FROM category WHERE category.parent_category_id = %d", req.TopCategory)
		} else if category.Level == 3 {
			sql = fmt.Sprintf("SELECT id FROM category WHERE id = %d", req.TopCategory)
		}
		query = query.Where(fmt.Sprintf("category_id in (%s)", sql)).Find(&goods)
	}
	var total int64
	query.Count(&total)
	goodsListResponse.Total = int32(total)

	// 把每一个good的预读取对象查到
	result := query.Preload("Category").Preload("Brands").Scopes(Paginate(int(req.Pages), int(req.PagePerNums))).Find(&goods)
	if result.Error != nil {
		return nil, result.Error
	}
	for _, good := range goods {
		goodsInfoResponce := ModelToResponse(good)
		goodsListResponse.Data = append(goodsListResponse.Data, &goodsInfoResponce)
	}

	return goodsListResponse, nil
}

// 非常重要的方法，根据筛选条件查商品
// 使用es完成查询
func (s *GoodsServer) GoodsList(ctx context.Context, req *proto.GoodsFilterRequest) (*proto.GoodsListResponse, error) {
	//关键词搜索、查询新品、查询热门商品、通过价格区间筛选， 通过商品分类筛选
	goodsListResponse := &proto.GoodsListResponse{}

	//match bool 复合查询
	q := elastic.NewBoolQuery()
	localDB := global.DB.Model(model.Goods{})
	if req.KeyWords != "" {
		// keywords查到了应该增加评分，用must
		// 在name和goods_brief里面都应该查，用Multi_Match
		q = q.Must(elastic.NewMultiMatchQuery(req.KeyWords, "name", "goods_brief"))
	}
	// 下面这几个是过滤查询，不增加评分
	if req.IsHot {
		localDB = localDB.Where(model.Goods{IsHot: true})
		q = q.Filter(elastic.NewTermQuery("is_hot", req.IsHot))
	}
	if req.IsNew {
		q = q.Filter(elastic.NewTermQuery("is_new", req.IsNew))
	}

	if req.PriceMin > 0 {
		q = q.Filter(elastic.NewRangeQuery("shop_price").Gte(req.PriceMin))
	}
	if req.PriceMax > 0 {
		q = q.Filter(elastic.NewRangeQuery("shop_price").Lte(req.PriceMax))
	}

	if req.Brand > 0 {
		q = q.Filter(elastic.NewTermQuery("brands_id", req.Brand))
	}

	//通过category去查询商品
	// 首先还是通过mysql把这个分类下所有商品id都查到
	var subQuery string
	categoryIds := make([]any, 0)
	if req.TopCategory > 0 {
		var category model.Category
		if result := global.DB.First(&category, req.TopCategory); result.RowsAffected == 0 {
			return nil, status.Errorf(codes.NotFound, "商品分类不存在")
		}

		if category.Level == 1 {
			subQuery = fmt.Sprintf("select id from category where parent_category_id in (select id from category WHERE parent_category_id=%d)", req.TopCategory)
		} else if category.Level == 2 {
			subQuery = fmt.Sprintf("select id from category WHERE parent_category_id=%d", req.TopCategory)
		} else if category.Level == 3 {
			subQuery = fmt.Sprintf("select id from category WHERE id=%d", req.TopCategory)
		}

		// 这一步只想要商品的id
		type Result struct {
			ID int32
		}
		var results []Result
		global.DB.Model(model.Category{}).Raw(subQuery).Scan(&results)
		for _, re := range results {
			categoryIds = append(categoryIds, re.ID)
		}

		//生成terms查询
		q = q.Filter(elastic.NewTermsQuery("category_id", categoryIds...))
	}

	//现在要的数据都查到了，还要分页
	if req.Pages == 0 {
		req.Pages = 1
	}

	switch {
	case req.PagePerNums > 100:
		req.PagePerNums = 100
	case req.PagePerNums <= 0:
		req.PagePerNums = 10
	}
	result, err := global.EsClient.Search().Index(model.EsGoods{}.GetIndexName()).Query(q).From(int(req.Pages)).Size(int(req.PagePerNums)).Do(context.Background())
	if err != nil {
		return nil, err
	}

	// 分页也分好了，但是商品还有一些字段是es的表没存的，还要再从mysql查一遍
	// 拿到全部的id，然后再mysql中查到，处理好返回
	goodsIds := make([]int32, 0)
	goodsListResponse.Total = int32(result.Hits.TotalHits.Value)
	for _, value := range result.Hits.Hits {
		goods := model.EsGoods{}
		_ = json.Unmarshal(value.Source, &goods)
		goodsIds = append(goodsIds, goods.ID)
	}

	// 把每一个good的预读取对象查到
	var goods []model.Goods
	re := localDB.Preload("Category").Preload("Brands").Find(&goods, goodsIds)
	if re.Error != nil {
		return nil, re.Error
	}

	for _, good := range goods {
		goodsInfoResponse := ModelToResponse(good)
		goodsListResponse.Data = append(goodsListResponse.Data, &goodsInfoResponse)
	}

	return goodsListResponse, nil
}

// 现在用户提交订单有多个商品，你得批量查询商品的信息吧
func (s *GoodsServer) BatchGetGoods(ctx context.Context, req *proto.BatchGoodsIdInfo) (*proto.GoodsListResponse, error) {
	goodsListResponse := &proto.GoodsListResponse{}
	var goods []model.Goods

	// 要批量查的条件如果是id，可以直接把切片放进where
	result := global.DB.Where(req.Id).Find(&goods)
	for _, good := range goods {
		goodsInfoResponse := ModelToResponse(good)
		goodsListResponse.Data = append(goodsListResponse.Data, &goodsInfoResponse)
	}
	goodsListResponse.Total = int32(result.RowsAffected)
	return goodsListResponse, nil
}

// 查询单个商品
func (s *GoodsServer) GetGoodsDetail(ctx context.Context, req *proto.GoodInfoRequest) (*proto.GoodsInfoResponse, error) {
	var goods model.Goods

	if result := global.DB.Preload("Category").Preload("Brands").First(&goods, req.Id); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "商品不存在")
	}
	goodsInfoResponse := ModelToResponse(goods)
	return &goodsInfoResponse, nil
}

func (s *GoodsServer) CreateGoods(ctx context.Context, req *proto.CreateGoodsInfo) (*proto.GoodsInfoResponse, error) {
	var category model.Category
	if result := global.DB.First(&category, req.CategoryId); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "商品分类不存在")
	}

	var brand model.Brands
	if result := global.DB.First(&brand, req.BrandId); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "品牌不存在")
	}
	// 关于图片这些文件是怎么上传的？其实像Gin这种框架是有文件上传功能的
	// 但是在微服务架构中，普通的文件上传是不能用的，进行到这一步理解为图片已经上传，以及拿到url了
	//先检查redis中是否有这个token
	//防止同一个token的数据重复插入到数据库中，如果redis中没有这个token则放入redis
	goods := model.Goods{
		Brands:          brand,
		BrandsID:        brand.ID,
		Category:        category,
		CategoryID:      category.ID,
		Name:            req.Name,
		GoodsSn:         req.GoodsSn,
		MarketPrice:     req.MarketPrice,
		ShopPrice:       req.ShopPrice,
		GoodsBrief:      req.GoodsBrief,
		ShipFree:        req.ShipFree,
		Images:          req.Images,
		DescImages:      req.DescImages,
		GoodsFrontImage: req.GoodsFrontImage,
		IsNew:           req.IsNew,
		IsHot:           req.IsHot,
		OnSale:          req.OnSale,
	}

	//srv之间互相调用了
	tx := global.DB.Begin()
	result := tx.Save(&goods)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}
	tx.Commit()
	return &proto.GoodsInfoResponse{
		Id: goods.ID,
	}, nil
}

func (s *GoodsServer) DeleteGoods(ctx context.Context, req *proto.DeleteGoodsInfo) (*emptypb.Empty, error) {
	if result := global.DB.Delete(&model.Goods{BaseModel: model.BaseModel{ID: req.Id}}, req.Id); result.Error != nil {
		return nil, status.Errorf(codes.NotFound, "商品不存在")
	}
	return &emptypb.Empty{}, nil
}

func (s *GoodsServer) UpdateGoods(ctx context.Context, req *proto.CreateGoodsInfo) (*emptypb.Empty, error) {
	var goods model.Goods

	if result := global.DB.First(&goods, req.Id); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "商品不存在")
	}

	var category model.Category
	if result := global.DB.First(&category, req.CategoryId); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "商品分类不存在")
	}

	var brand model.Brands
	if result := global.DB.First(&brand, req.BrandId); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "品牌不存在")
	}

	goods.Brands = brand
	goods.BrandsID = brand.ID
	goods.Category = category
	goods.CategoryID = category.ID
	goods.Name = req.Name
	goods.GoodsSn = req.GoodsSn
	goods.MarketPrice = req.MarketPrice
	goods.ShopPrice = req.ShopPrice
	goods.GoodsBrief = req.GoodsBrief
	goods.ShipFree = req.ShipFree
	goods.Images = req.Images
	goods.DescImages = req.DescImages
	goods.GoodsFrontImage = req.GoodsFrontImage
	goods.IsNew = req.IsNew
	goods.IsHot = req.IsHot
	goods.OnSale = req.OnSale

	tx := global.DB.Begin()
	result := tx.Save(&goods)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}
	tx.Commit()
	return &emptypb.Empty{}, nil
}
