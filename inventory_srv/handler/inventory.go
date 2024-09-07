package handler

import (
	"LingFengShop/inventory_srv/global"
	"LingFengShop/inventory_srv/model"
	"LingFengShop/inventory_srv/proto"
	"context"
	"encoding/json"
	"fmt"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

type InventoryServer struct {
	proto.UnimplementedInventoryServer
}

// mustEmbedUnimplementedUserServer implements proto.UserServer.
func (s *InventoryServer) mustEmbedUnimplementedUserServer() {
	panic("unimplemented")
}

// 设置商品库存
func (*InventoryServer) SetInv(ctx context.Context, req *proto.GoodsInvInfo) (*emptypb.Empty, error) {
	//设置库存， 如果我要更新库存
	var inv model.Inventory
	global.DB.Where(&model.Inventory{Goods: req.GoodsId}).First(&inv)
	inv.Goods = req.GoodsId
	inv.Stocks = req.Num

	global.DB.Save(&inv)
	return &emptypb.Empty{}, nil
}

// 获取库存
func (*InventoryServer) InvDetail(ctx context.Context, req *proto.GoodsInvInfo) (*proto.GoodsInvInfo, error) {
	var inv model.Inventory
	if result := global.DB.Where(&model.Inventory{Goods: req.GoodsId}).First(&inv); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "没有库存信息")
	}
	return &proto.GoodsInvInfo{
		GoodsId: inv.Goods,
		Num:     inv.Stocks,
	}, nil
}

// var m sync.Mutex

// 减少库存
func (*InventoryServer) Sell(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	// 一个购物车提交订单，一批商品减库存要么都成功，要么都失败，需要使用事务
	// 用事务仅仅只解决了单个请求的问题，并发来了，多个事务同时启动
	// 依然会出现并发问题：数据一致性无法保证（如超卖）
	// 要保证并发安全，用一把全局的锁，每个事务都放在这把锁里
	// m.Lock()

	// 每次扣减库存，还要在库存扣减表里加一条数据
	sellDetail := model.StockSellDetail{
		OrderSn: req.OrderSn,
		Status:  0, // 现在还没扣减，状态暂时写0，扣减了就改成1
		Detail:  nil,
	}
	// 这里放商品扣减细节
	details := []model.GoodsDetail{}

	tx := global.DB.Begin()
	for _, goodsInfo := range req.GoodsInfo {
		details = append(details, model.GoodsDetail{
			Goods: goodsInfo.GoodsId,
			Num:   goodsInfo.Num,
		})
		var info model.Inventory
		// gorm中，只要事务不提交，就可以做到autocommit=0
		// 只需要用tx加一个Clauses(clause.Locking{Strength: "UPDATE"})就可以
		// 悲观锁：
		// if res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(&model.Inventory{Goods: goodsInfo.GoodsId}).First(&info); res.RowsAffected == 0 {
		// 	tx.Rollback() // 回滚
		// 	return nil, status.Errorf(codes.InvalidArgument, "没有库存信息")
		// }
		// for {
		// 乐观锁：
		mutexname := fmt.Sprintf("goods_%d", goodsInfo.GoodsId)
		mutex := global.RS.NewMutex(mutexname)
		if err := mutex.Lock(); err != nil {
			fmt.Println(err)
			return nil, status.Errorf(codes.Internal, "获取redis分布式锁异常")
		}
		if res := global.DB.Where(&model.Inventory{Goods: goodsInfo.GoodsId}).First(&info); res.RowsAffected == 0 {
			tx.Rollback() // 回滚
			return nil, status.Errorf(codes.InvalidArgument, "没有库存信息")
		}
		if info.Stocks < goodsInfo.Num {
			tx.Rollback() // 回滚
			return nil, status.Errorf(codes.ResourceExhausted, "库存不足")
		}
		// 可以扣减了
		info.Stocks -= goodsInfo.Num
		tx.Save(&info)
		// 扣减状态改成1
		sellDetail.Status = 1

		if ok, err := mutex.Unlock(); !ok || err != nil {
			return nil, status.Errorf(codes.Internal, "释放redis分布式锁异常")
		}
		// 零值 注意：int类型默认0，gorm中对应零值默认忽略
		// 如果stock是0，gorm会忽略本次更改，想强制更改，加select("想强制还的字段")
		// res := tx.Model(&model.Inventory{}).Select("Stocks", "Version").Where("goods = ? and version = ?", goodsInfo.GoodsId, info.Version).
		// 	Updates(&model.Inventory{Stocks: info.Stocks, Version: info.Version + 1})
		// if res.RowsAffected == 0 {
		// 	zap.S().Info("库存扣减失败")
		// } else {
		// 	break
		// }
		// }
		// tx.Save(&info)
	}
	sellDetail.Detail = details
	// 传入sellDetail表
	if res := tx.Create(&sellDetail); res.Error != nil {
		tx.Rollback() // 回滚
		return nil, status.Errorf(codes.Internal, "保存库存扣减历史失败")
	}
	tx.Commit() // 需要手动提交

	// m.Unlock()
	return &emptypb.Empty{}, nil
}

// 归还库存
func (*InventoryServer) Reback(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	//库存归还： 1：订单超时归还 2. 订单创建失败，归还之前扣减的库存 3. 手动归还
	// 和sell方法的最初版本一样，使用事务
	tx := global.DB.Begin()
	for _, goodInfo := range req.GoodsInfo {
		var inv model.Inventory
		if result := global.DB.Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
			tx.Rollback()
			return nil, status.Errorf(codes.InvalidArgument, "没有库存信息")
		}

		inv.Stocks += goodInfo.Num
		tx.Save(&inv)
	}
	tx.Commit()
	return &emptypb.Empty{}, nil
}

// 自动归还库存
func AutoReback(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	type OrderInfo struct {
		OrderSn string
	}
	// 可能有多个归还的消息
	for i := range msgs {
		//既然是归还库存，那么我应该具体的知道每件商品应该归还多少， 但是有一个问题是什么？重复归还的问题
		//所以说这个接口应该确保幂等性， 你不能因为消息的重复发送导致一个订单的库存归还多次， 没有扣减的库存你别归还
		//如果确保这些都没有问题， 新建一张表， 这张表记录了详细的订单扣减细节，以及归还细节
		var orderInfo OrderInfo
		err := json.Unmarshal(msgs[i].Body, &orderInfo)
		if err != nil {
			zap.S().Errorf("解析json失败： %v\n", msgs[i].Body)
			// 返回ConsumeSuccess表示直接丢弃这个消息
			return consumer.ConsumeSuccess, nil
		}

		//去将inv的库存加回去 将selldetail的status设置为2， 要在事务中进行
		tx := global.DB.Begin()
		var sellDetail model.StockSellDetail
		if result := tx.Model(&model.StockSellDetail{}).
			Where(&model.StockSellDetail{OrderSn: orderInfo.OrderSn, Status: 1}). // 直接找没归还的同一个订单，没找到说明归还了
			First(&sellDetail); result.RowsAffected == 0 {
			return consumer.ConsumeSuccess, nil
		}
		//逐个归还库存
		for _, orderGood := range sellDetail.Detail {
			//update怎么用
			//先查询一下inventory表在， update语句的 update xx set stocks=stocks+2
			// 用gorm的update原子操作语法
			if result := tx.Model(&model.Inventory{}).
				Where(&model.Inventory{Goods: orderGood.Goods}).
				Update("stocks", gorm.Expr("stocks+?", orderGood.Num)); result.RowsAffected == 0 {
				tx.Rollback()
				//如果没归还成功，过一会重试
				return consumer.ConsumeRetryLater, nil
			}
		}

		// 改存款扣减表，把状态设为2（已归还）
		if result := tx.Model(&model.StockSellDetail{}).
			Where(&model.StockSellDetail{OrderSn: orderInfo.OrderSn}).
			Update("status", 2); result.RowsAffected == 0 {
			tx.Rollback()
			return consumer.ConsumeRetryLater, nil
		}
		tx.Commit()
		return consumer.ConsumeSuccess, nil
	}
	return consumer.ConsumeSuccess, nil
}
