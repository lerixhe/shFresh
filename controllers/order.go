package controllers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	"github.com/gomodule/redigo/redis"
	"log"
	"shFresh/models"
	"shFresh/redispool"
	"strconv"
	"strings"
	"time"
)

type OrderController struct {
	beego.Controller
}

func (this *OrderController) ShowOrder() {
	this.TplName = "place_order.html"
	// 获取数据
	userName := GetUser(&this.Controller)
	skuids := this.GetStrings("skuid")
	log.Println("订单页面获取到商品：", skuids)
	// 校验数据
	if len(skuids) == 0 {
		log.Println("订单数据为空,请检查购物车")
		this.Redirect("/user/cart", 302)
	}

	// 处理数据
	conn := redispool.Redisclient.Get()
	defer conn.Close()
	o := orm.NewOrm()
	user := models.User{Name: userName}
	o.Read(&user, "Name")
	goodsBuffer := make([]map[string]interface{}, len(skuids))
	totalPrice := 0
	totalCount := 0
	for index, value := range skuids {
		temp := make(map[string]interface{})
		skuid, err := strconv.Atoi(value)
		if err != nil {
			log.Println("商品id不合法", err)
			this.Redirect("/user/cart", 302)
			return
		}
		goodsSKU := models.GoodsSKU{Id: skuid}
		o.Read(&goodsSKU)
		temp["goodssku"] = goodsSKU
		count, err := redis.Int(conn.Do("hget", "cart_"+strconv.Itoa(user.Id), skuid))
		if err != nil {
			log.Println("商品数量错误", err)
			this.Redirect("/user/cart", 302)
			return
		}
		temp["count"] = count
		totalCount += count
		// 商品序号
		temp["index"] = index + 1
		// 商品小计
		temp["amount"] = goodsSKU.Price * count
		totalPrice += goodsSKU.Price * count
		goodsBuffer[index] = temp
	}
	this.Data["goodsBuffer"] = goodsBuffer
	this.Data["totalPrice"] = totalPrice
	this.Data["totalCount"] = totalCount
	transferPrice := 10
	this.Data["transferPrice"] = transferPrice
	discount := 18
	this.Data["discount"] = discount
	actualPayment := totalPrice + transferPrice - discount
	if actualPayment <= 0 {
		actualPayment = 0
		// 实付款不能小于0
	}
	this.Data["actualPayment"] = actualPayment
	// 传递所有商品id
	this.Data["skuids"] = skuids

	// 处理收货地址
	addrs := []models.Address{}
	o.QueryTable("Address").RelatedSel("User").Filter("User__id", user.Id).All(&addrs)
	this.Data["addrs"] = addrs

}

// 创建订单
func (this *OrderController) AddOrder() {
	// 获取数据
	addrId, _ := this.GetInt("addrId")
	payId, _ := this.GetInt("payId")
	skuidString := this.GetString("skuids")
	// 获取到的skuidsString为类型切片形式的字符串类型，需要进行剪裁转换为字符串
	ids := skuidString[1 : len(skuidString)-1]
	skuids := strings.Fields(ids)
	log.Println("用户提交订单中的商品iD", skuids)
	totalCount, _ := this.GetInt("totalCount")
	totalPrice, _ := this.GetInt("totalPrice")
	discount, _ := this.GetInt("discount")
	transit, _ := this.GetInt("transit")
	// actualPayment, _ := this.GetInt("actualPayment")
	userName := this.GetSession("userName")
	resp := make(map[string]interface{})
	defer this.ServeJSON()
	// 校验数据
	if len(skuids) == 0 {
		resp["code"] = 1
		resp["msg"] = "订单商品数据不合法"
		log.Println("订单商品数据不合法")
		this.Data["json"] = resp
		return
	}
	if userName == nil {
		resp["code"] = 2
		resp["msg"] = "用户未登录"
		log.Println("用户未登录")
		this.Data["json"] = resp
		return
	}
	// 处理数据：
	// 1.向订单表中插入数据
	// 需注意订单整个流程为事物操作
	o := orm.NewOrm()
	o.Begin()
	user := models.User{Name: userName.(string)}
	o.Read(&user, "Name")
	addr := models.Address{Id: addrId}
	o.Read(&addr, "Id")
	order := models.OrderInfo{
		User:         &user,
		Address:      &addr,
		PayMethod:    payId,
		TotalCount:   totalCount,
		TotalPrice:   totalPrice,
		TransitPrice: transit,
		Discount:     discount,
		Orderstatus:  1,
	}
	order.OrderId = time.Now().Format("20060102150405") + strconv.Itoa(user.Id)
	// 2.执行插入操作
	_, err := o.Insert(&order)
	if err != nil {
		resp["code"] = 3
		resp["msg"] = "订单生成失败"
		this.Data["json"] = resp
		o.Rollback()
		log.Println("订单生成失败,已回滚:", err)
		return
	}
	// 向订单商品表中插入数据
	conn := redispool.Redisclient.Get()
	defer conn.Close()
	for _, value := range skuids {
		skuid, _ := strconv.Atoi(string(value))
		log.Println(skuid)
		goods := models.GoodsSKU{Id: skuid}
		o.Read(&goods)
		orderGoods := models.OrderGoods{
			GoodsSKU:  &goods,
			OrderInfo: &order,
		}
		count, err := redis.Int(conn.Do("hget", "cart_"+strconv.Itoa(user.Id), skuid))
		if err != nil {
			resp["code"] = 3
			resp["msg"] = "订单商品获取失败"
			o.Rollback()
			log.Println("订单商品获取失败,已回滚：", err)
			this.Data["json"] = resp
			return
		}
		// 判断库存
		if count > goods.Stock {
			// 库存不足
			resp["code"] = 4
			resp["msg"] = "存在库存不足的商品，请返回购物车查看"
			o.Rollback()
			log.Println("存在库存不足的商品.操作已回滚，信息如下：", err)
			log.Printf("商品id:%d,库存数量：%d,所需数量：%d", skuid, goods.Stock, count)
			this.Data["json"] = resp
			return
		}
		orderGoods.Count = count
		orderGoods.Price = count * goods.Price
		// 执行单个插入
		o.Insert(&orderGoods)
	}
	// 操作成功，返回成功信息
	resp["code"] = 200
	resp["msg"] = "OK"
	o.Commit()
	log.Println("订单创建成功！")
	this.Data["json"] = resp
	return
}
