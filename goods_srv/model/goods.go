package model

import (
	"LingFengShop/goods_srv/global"
	"context"
	"strconv"

	"gorm.io/gorm"
)

// 分类表
// 实际开发中，类型尽量设置不可以为null
type Category struct {
	BaseModel
	Name             string    `gorm:"type:varchar(20);not null"`
	Level            int32     `gorm:"type:int;not null;default:1"` // 设置几级分类
	IsTab            bool      `gorm:"default:false;not null"`      // 是否能显示在tab栏中
	ParentCategoryID int32     `json:"parent"`                      // json取值的时候变成这个parent
	ParentCategory   *Category `json:"-"`                           //外键对象，希望json取值时忽略这个字段，写-即可
	// 下面这个外键的预加载对象，通过foreignKey指明是哪个外键，references指明ID
	SubCategory []*Category `gorm:"foreignKey:ParentCategoryID;references:ID" json:"sub_category"`
}

// 品牌
type Brands struct {
	BaseModel
	Name string `gorm:"type:varchar(50);not null"`
	Logo string `gorm:"type:varchar(200);default:'';not null"`
}

// 品牌和分类是m:n关系，这张表用来连接两张表
// 加上联合索引
type GoodsCategoryBrand struct {
	BaseModel
	CategoryID int32 `gorm:"type:int;index:idx_category_brand,unique"`
	Category   Category

	BrandsID int32 `gorm:"type:int;index:idx_category_brand,unique"`
	Brands   Brands
}

// 重载表名，默认是下划线，不想下划线可以自定义
func (GoodsCategoryBrand) TableName() string {
	return "goodscategorybrand"
}

// 轮播图表
type Banner struct {
	BaseModel
	Image string `gorm:"type:varchar(200);not null"`  //图片
	Url   string `gorm:"type:varchar(200);not null"`  //商品页的url，可点击跳转到商品
	Index int32  `gorm:"type:int;default:1;not null"` // 轮播图的先后顺序
}

// 商品
type Goods struct {
	BaseModel

	CategoryID int32 `gorm:"type:int;not null"`
	Category   Category
	BrandsID   int32 `gorm:"type:int;not null"`
	Brands     Brands

	OnSale   bool `gorm:"default:false;not null"`
	ShipFree bool `gorm:"default:false;not null"` // 是否免运费
	IsNew    bool `gorm:"default:false;not null"` // 是否是新品
	IsHot    bool `gorm:"default:false;not null"` // 是否热卖，广告位

	Name     string `gorm:"type:varchar(50);not null"`
	GoodsSn  string `gorm:"type:varchar(50);not null"` // 商家自己的编号，商家自己有一套管理系统
	ClickNum int32  `gorm:"type:int;default:0;not null"`
	SoldNum  int32  `gorm:"type:int;default:0;not null"` // 这三个都用于分析
	FavNum   int32  `gorm:"type:int;default:0;not null"`

	MarketPrice float32 `gorm:"not null"`
	ShopPrice   float32 `gorm:"not null"`
	GoodsBrief  string  `gorm:"type:varchar(100);not null"` // 商品简介
	// 数据库里没有数组类型，模型字段不能是切片
	Images          GormList `gorm:"type:varchar(1000);not null"`
	DescImages      GormList `gorm:"type:varchar(1000);not null"`
	GoodsFrontImage string   `gorm:"type:varchar(200);not null"`
}

// 要保持es和mysql的数据一致性，使用gorm的钩子方法
func (g *Goods) AfterCreate(tx *gorm.DB) (err error) {
	esModel := EsGoods{
		ID:          g.ID,
		CategoryID:  g.CategoryID,
		BrandsID:    g.BrandsID,
		OnSale:      g.OnSale,
		ShipFree:    g.ShipFree,
		IsNew:       g.IsNew,
		IsHot:       g.IsHot,
		Name:        g.Name,
		ClickNum:    g.ClickNum,
		SoldNum:     g.SoldNum,
		FavNum:      g.FavNum,
		MarketPrice: g.MarketPrice,
		GoodsBrief:  g.GoodsBrief,
		ShopPrice:   g.ShopPrice,
	}

	_, err = global.EsClient.Index().Index(esModel.GetIndexName()).BodyJson(esModel).Id(strconv.Itoa(int(g.ID))).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (g *Goods) AfterUpdate(tx *gorm.DB) (err error) {
	esModel := EsGoods{
		ID:          g.ID,
		CategoryID:  g.CategoryID,
		BrandsID:    g.BrandsID,
		OnSale:      g.OnSale,
		ShipFree:    g.ShipFree,
		IsNew:       g.IsNew,
		IsHot:       g.IsHot,
		Name:        g.Name,
		ClickNum:    g.ClickNum,
		SoldNum:     g.SoldNum,
		FavNum:      g.FavNum,
		MarketPrice: g.MarketPrice,
		GoodsBrief:  g.GoodsBrief,
		ShopPrice:   g.ShopPrice,
	}

	_, err = global.EsClient.Update().Index(esModel.GetIndexName()).
		Doc(esModel).Id(strconv.Itoa(int(g.ID))).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (g *Goods) AfterDelete(tx *gorm.DB) (err error) {
	_, err = global.EsClient.Delete().Index(EsGoods{}.GetIndexName()).Id(strconv.Itoa(int(g.ID))).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}
