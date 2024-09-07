package handler

import (
	"LingFengShop/order_srv/global"
	"LingFengShop/order_srv/model"
	"LingFengShop/order_srv/proto"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"golang.org/x/exp/rand"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type OrderServer struct {
	proto.UnimplementedOrderServer
}

// 生成订单号
func GenerateOrderSn(userId int32) string {
	//订单号的生成规则
	/*
		年月日时分秒+用户id+2位随机数
	*/
	now := time.Now()
	rand.New(rand.NewSource(uint64(time.Now().UnixNano())))
	orderSn := fmt.Sprintf("%d%d%d%d%d%d%d%d",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Nanosecond(),
		userId, rand.Intn(90)+10,
	)
	return orderSn
}

// 获取用户的购物车列表
func (*OrderServer) CartItemList(ctx context.Context, req *proto.UserInfo) (*proto.CartItemListResponse, error) {
	var shopCarts []model.ShoppingCart
	var rsp proto.CartItemListResponse

	if result := global.DB.Where(&model.ShoppingCart{User: req.Id}).Find(&shopCarts); result.Error != nil {
		return nil, result.Error
	} else {
		rsp.Total = int32(result.RowsAffected)
	}

	for _, shopCart := range shopCarts {
		rsp.Data = append(rsp.Data, &proto.ShopCartInfoResponse{
			Id:      shopCart.ID,
			UserId:  shopCart.User,
			GoodsId: shopCart.Goods,
			Nums:    shopCart.Nums,
			Checked: shopCart.Checked,
		})
	}
	return &rsp, nil
}

// 将商品添加到购物车
func (*OrderServer) CreateCartItem(ctx context.Context, req *proto.CartItemRequest) (*proto.ShopCartInfoResponse, error) {
	var shopCart model.ShoppingCart

	//为了严谨性，添加商品到购物车之前，记得检查一下商品是否存在
	_, err := global.GoodsSrvClient.GetGoodsDetail(context.Background(), &proto.GoodInfoRequest{
		Id: req.GoodsId,
	})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "商品不存在")
	}

	if result := global.DB.Where(&model.ShoppingCart{Goods: req.GoodsId, User: req.UserId}).First(&shopCart); result.RowsAffected == 1 {
		//这个商品之前已经添加到了购物车- 合并
		shopCart.Nums += req.Nums
	} else {
		//购物车中原本没有这件商品 - 新建一个记录
		shopCart.User = req.UserId
		shopCart.Goods = req.GoodsId
		shopCart.Nums = req.Nums
		shopCart.Checked = false
	}

	global.DB.Save(&shopCart)
	return &proto.ShopCartInfoResponse{Id: shopCart.ID}, nil
}

// 更新购物车记录，更新数量和选中状态
func (*OrderServer) UpdateCartItem(ctx context.Context, req *proto.CartItemRequest) (*emptypb.Empty, error) {
	var shopCart model.ShoppingCart

	if result := global.DB.Where("goods=? and user=?", req.GoodsId, req.UserId).First(&shopCart); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "购物车记录不存在")
	}

	shopCart.Checked = req.Checked
	if req.Nums > 0 {
		shopCart.Nums = req.Nums
	}
	global.DB.Save(&shopCart)

	return &emptypb.Empty{}, nil
}

func (*OrderServer) DeleteCartItem(ctx context.Context, req *proto.CartItemRequest) (*emptypb.Empty, error) {
	if result := global.DB.Where("goods=? and user=?", req.GoodsId, req.UserId).Delete(&model.ShoppingCart{}); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "购物车记录不存在")
	}
	return &emptypb.Empty{}, nil
}

// 获取订单列表
func (*OrderServer) OrderList(ctx context.Context, req *proto.OrderFilterRequest) (*proto.OrderListResponse, error) {
	var orders []model.OrderInfo
	var rsp proto.OrderListResponse

	var total int64
	global.DB.Where(&model.OrderInfo{User: req.UserId}).Count(&total)
	rsp.Total = int32(total)

	//分页
	global.DB.Scopes(Paginate(int(req.Pages), int(req.PagePerNums))).Where(&model.OrderInfo{User: req.UserId}).Find(&orders)
	for _, order := range orders {
		rsp.Data = append(rsp.Data, &proto.OrderInfoResponse{
			Id:      order.ID,
			UserId:  order.User,
			OrderSn: order.OrderSn,
			PayType: order.PayType,
			Status:  order.Status,
			Post:    order.Post,
			Total:   order.OrderMount,
			Address: order.Address,
			Name:    order.SignerName,
			Phone:   order.SingerPhone,
			AddTime: order.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return &rsp, nil
}

// 获取订单详情
func (*OrderServer) OrderDetail(ctx context.Context, req *proto.OrderRequest) (*proto.OrderInfoDetailResponse, error) {
	var order model.OrderInfo
	var rsp proto.OrderInfoDetailResponse

	//这个订单的id是否是当前用户的订单， 如果在web层用户传递过来一个id的订单， web层应该先查询一下订单id是否是当前用户的
	//在个人中心可以这样做，但是如果是后台管理系统，web层如果是后台管理系统 那么只传递order的id，如果是电商系统还需要一个用户的id
	if result := global.DB.Where(&model.OrderInfo{BaseModel: model.BaseModel{ID: req.Id}, User: req.UserId}).First(&order); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "订单不存在")
	}

	orderInfo := proto.OrderInfoResponse{}
	orderInfo.Id = order.ID
	orderInfo.UserId = order.User
	orderInfo.OrderSn = order.OrderSn
	orderInfo.PayType = order.PayType
	orderInfo.Status = order.Status
	orderInfo.Post = order.Post
	orderInfo.Total = order.OrderMount
	orderInfo.Address = order.Address
	orderInfo.Name = order.SignerName
	orderInfo.Phone = order.SingerPhone

	rsp.OrderInfo = &orderInfo

	// rsp还需要包含该订单的所有商品信息
	var orderGoods []model.OrderGoods
	if result := global.DB.Where(&model.OrderGoods{Order: order.ID}).Find(&orderGoods); result.Error != nil {
		return nil, result.Error
	}

	for _, orderGood := range orderGoods {
		rsp.Goods = append(rsp.Goods, &proto.OrderItemResponse{
			GoodsId:    orderGood.Goods,
			GoodsName:  orderGood.GoodsName,
			GoodsPrice: orderGood.GoodsPrice,
			GoodsImage: orderGood.GoodsImage,
			Nums:       orderGood.Nums,
		})
	}

	return &rsp, nil
}

type OrderListener struct {
	Code        codes.Code // 用这个Code来表示订单本地逻辑成没成功
	Detail      string
	ID          int32
	OrderAmount float32
	Ctx         context.Context // 通过这个ctx用来传递span
}

// 执行本地逻辑
/*
	新建订单
		1. 从购物车中获取到选中的商品
		2. 商品的价格自己查询 - 访问商品服务 (跨微服务)
		3. 库存的扣减 - 访问库存服务 (跨微服务)
		4. 生成订单表
		5. 从购物车中删除已购买的记录
*/
func (o *OrderListener) ExecuteLocalTransaction(msg *primitive.Message) primitive.LocalTransactionState {
	// 先拿到span
	parentSpan := opentracing.SpanFromContext(o.Ctx)
	// 接收mq消息，反解order，消息自动放在msg里面
	var orderInfo *model.OrderInfo
	_ = json.Unmarshal(msg.Body, &orderInfo)
	// 1.
	var shopCarts []model.ShoppingCart
	// 这个表记录购物车里所有商品id对应的数量，方便后面计算价格
	goodsNumMap := make(map[int32]int32)
	shopCartSpan := opentracing.GlobalTracer().StartSpan("select_shopcart", opentracing.ChildOf(parentSpan.Context()))
	if result := global.DB.Where(&model.ShoppingCart{User: orderInfo.User, Checked: true}).Find(&shopCarts); result.RowsAffected == 0 {
		o.Code = codes.InvalidArgument
		o.Detail = "没有选择结算的商品"
		return primitive.RollbackMessageState //还没grpc调扣库存服务，可以直接rollback
	}
	shopCartSpan.Finish()

	// 2.
	var goodsIds []int32
	for _, shopCart := range shopCarts {
		goodsIds = append(goodsIds, shopCart.Goods)
		goodsNumMap[shopCart.Goods] = shopCart.Nums
	}
	// 跨服务调用 - gin
	goodsQuerySpan := opentracing.GlobalTracer().StartSpan("query_goods", opentracing.ChildOf(parentSpan.Context()))
	goods, err := global.GoodsSrvClient.BatchGetGoods(context.Background(), &proto.BatchGoodsIdInfo{Id: goodsIds})
	if err != nil {
		o.Code = codes.Internal
		o.Detail = "查询商品信息失败"
		return primitive.RollbackMessageState //还没grpc调扣库存服务，可以直接rollback
	}
	goodsQuerySpan.Finish()
	// 总价
	var orderAmount float32
	// 新建订单，OrderGoods表也要更新，把要的新增的建成一个切片
	var orderGoods []*model.OrderGoods
	var goodsInvInfo []*proto.GoodsInvInfo
	for _, good := range goods.Data {
		orderAmount += float32(good.ShopPrice) * float32(goodsNumMap[good.Id])
		orderGoods = append(orderGoods, &model.OrderGoods{
			BaseModel:  model.BaseModel{},
			Goods:      good.Id,
			GoodsName:  good.Name,
			GoodsImage: good.GoodsFrontImage,
			GoodsPrice: good.ShopPrice,
			Nums:       goodsNumMap[good.Id],
		})
		goodsInvInfo = append(goodsInvInfo, &proto.GoodsInvInfo{
			GoodsId: good.Id,
			Num:     goodsNumMap[good.Id],
		})
	}

	// 3.扣减库存
	sellSpan := opentracing.GlobalTracer().StartSpan("sell", opentracing.ChildOf(parentSpan.Context()))
	_, err = global.InventorySrvClient.Sell(context.Background(), &proto.SellInfo{
		OrderSn:   orderInfo.OrderSn,
		GoodsInfo: goodsInvInfo,
	})
	sellSpan.Finish()
	if err != nil {
		//如果是因为网络问题，这种如何避免误判，大家自己改写一下sell的返回逻辑
		// 返回的状态码如果不是我规定的几个之中的一个，就说明是网络问题
		o.Code = codes.ResourceExhausted
		o.Detail = "扣减库存失败"
		return primitive.RollbackMessageState
	}

	// 4.生成订单表
	saveOrderAndDeleteCart := opentracing.GlobalTracer().StartSpan("saveorder_deletecart", opentracing.ChildOf(parentSpan.Context()))
	tx := global.DB.Begin()
	orderInfo.OrderMount = orderAmount

	if res := tx.Save(&orderInfo); res.RowsAffected == 0 {
		tx.Rollback()
		o.Code = codes.Internal
		o.Detail = "创建订单失败"
		return primitive.CommitMessageState
	}

	o.OrderAmount = orderAmount
	o.ID = orderInfo.ID

	for _, orderGood := range orderGoods {
		orderGood.Order = orderInfo.ID
	}

	// 批量插入订单商品表
	if res := tx.CreateInBatches(orderGoods, 100); res.RowsAffected == 0 {
		tx.Rollback()
		o.Code = codes.Internal
		o.Detail = "批量插入订单商品失败"
		return primitive.CommitMessageState
	}
	// 从购物车里删除
	if res := tx.Where(&model.ShoppingCart{User: orderInfo.User, Checked: true}).Delete(&model.ShoppingCart{}); res.RowsAffected == 0 {
		tx.Rollback()
		o.Code = codes.Internal
		o.Detail = "删除购物车记录失败"
		return primitive.CommitMessageState
	}
	saveOrderAndDeleteCart.Finish()
	//发送延时消息，这个是用来解决订单超时归还的
	p, err := rocketmq.NewProducer(
		producer.WithNameServer([]string{"10.237.56.85:9876"}),
		producer.WithGroupName("time_out_check"),
	)
	if err != nil {
		panic("生成producer失败")
	}

	//不要在一个进程中使用多个producer， 但是不要随便调用shutdown因为会影响其他的producer
	if err = p.Start(); err != nil {
		panic("启动producer1失败")
	}

	msg = primitive.NewMessage("order_timeout", msg.Body)
	msg.WithDelayTimeLevel(3)
	_, err = p.SendSync(context.Background(), msg)
	if err != nil {
		zap.S().Errorf("发送延时消息失败: %v\n", err)
		tx.Rollback()
		o.Code = codes.Internal
		o.Detail = "发送延时消息失败"
		return primitive.CommitMessageState
	}

	if err = p.Shutdown(); err != nil {
		panic("关闭producer失败")
	}

	tx.Commit()
	// 订单本地的逻辑成功了
	o.Code = codes.OK
	return primitive.RollbackMessageState
}

// 消息回查
func (o *OrderListener) CheckLocalTransaction(msg *primitive.MessageExt) primitive.LocalTransactionState {
	var orderInfo model.OrderInfo
	_ = json.Unmarshal(msg.Body, &orderInfo)

	// 检查订单记录生成好没有？
	if result := global.DB.Where(model.OrderInfo{OrderSn: orderInfo.OrderSn}).First(&orderInfo); result.RowsAffected == 0 {
		// 没查到，说明本地没成功
		return primitive.CommitMessageState // 你并不能说明这里就是库存已经扣减了
	}

	return primitive.RollbackMessageState
}

// 新建订单
func (*OrderServer) CreateOrder(ctx context.Context, req *proto.OrderRequest) (*proto.OrderInfoResponse, error) {
	// 发送事务消息
	orderListener := &OrderListener{Ctx: ctx}
	p, err := rocketmq.NewTransactionProducer(
		orderListener,
		producer.WithNameServer([]string{"10.237.56.85:9876"}),
		producer.WithGroupName("send_reback_half"), // 每个producer和consumer的名称不要一样，否则报错
	)
	if err != nil {
		zap.S().Errorf("生成producer失败，%s", err.Error())
		return nil, err
	}

	if err = p.Start(); err != nil {
		zap.S().Errorf("启动producer失败，%s", err.Error())
		return nil, err
	}

	// 把订单信息作为半消息发出去
	order := model.OrderInfo{
		OrderSn:     GenerateOrderSn(req.UserId),
		Address:     req.Address,
		SignerName:  req.Name,
		SingerPhone: req.Phone,
		Post:        req.Post,
		User:        req.UserId,
	}
	jsonStr, _ := json.Marshal(order)

	_, err = p.SendMessageInTransaction(context.Background(),
		primitive.NewMessage("order_reback", []byte(jsonStr)))
	if err != nil {
		fmt.Printf("发送失败: %s\n", err)
		return nil, status.Error(codes.Internal, "半信息发送失败")
	}
	if err = p.Shutdown(); err != nil {
		panic("关闭producer失败")
	}
	// 检查Code，确认本地逻辑是否成功
	if orderListener.Code != codes.OK {
		return nil, status.Error(orderListener.Code, orderListener.Detail)
	}

	return &proto.OrderInfoResponse{Id: orderListener.ID, OrderSn: order.OrderSn, Total: orderListener.OrderAmount}, nil
}

// 更新订单状态
func (*OrderServer) UpdateOrderStatus(ctx context.Context, req *proto.OrderStatus) (*emptypb.Empty, error) {
	//先查询，再更新 实际上有两条sql执行， select 和 update语句
	if result := global.DB.Model(&model.OrderInfo{}).Where("order_sn = ?", req.OrderSn).Update("status", req.Status); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "订单不存在")
	}
	return &emptypb.Empty{}, nil
}

// 订单超时的处理，查询订单的支付状态，如果已支付什么都不做，如果未支付，归还库存
func OrderTimeout(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	// 有可能有多个订单超时处理，所以要range
	for i := range msgs {
		var orderInfo model.OrderInfo
		_ = json.Unmarshal(msgs[i].Body, &orderInfo)

		fmt.Printf("获取到订单超时消息: %v\n", time.Now())
		var order model.OrderInfo
		if result := global.DB.Model(model.OrderInfo{}).Where(model.OrderInfo{OrderSn: orderInfo.OrderSn}).First(&order); result.RowsAffected == 0 {
			// 没找到订单，什么都不做
			return consumer.ConsumeSuccess, nil
		}
		if order.Status != "TRADE_SUCCESS" || order.Status != "TRADE_CLOSED" {
			tx := global.DB.Begin()
			// 归还库存，我们可以模仿order中发送一个消息到 order_reback中，让库存服务归还库存
			// 修改订单的状态为关闭
			order.Status = "TRADE_CLOSED"
			tx.Save(&order)

			p, err := rocketmq.NewProducer(
				producer.WithNameServer([]string{"10.237.56.85:9876"}),
				producer.WithGroupName("timeout_reback"),
			)
			if err != nil {
				panic("生成producer失败")
			}

			if err = p.Start(); err != nil {
				panic("启动producer失败")
			}

			_, err = p.SendSync(context.Background(), primitive.NewMessage("order_reback", msgs[i].Body))
			if err != nil {
				tx.Rollback()
				fmt.Printf("发送失败: %s\n", err)
				return consumer.ConsumeRetryLater, nil
			}
			// 超时归还消息处理成功
			if err = p.Shutdown(); err != nil {
				panic("关闭producer失败")
			}
			tx.Commit()
			return consumer.ConsumeSuccess, nil
		}
	}
	return consumer.ConsumeSuccess, nil
}
