package controllers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	"github.com/gomodule/redigo/redis"
	"github.com/smartwalle/alipay"
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
		var count = 0
		source, _ := this.GetInt("source")
		log.Println("本次订单显示的请求来源(1商品详情  0购物车)：", source)
		// 判断请求来源
		if source == 1 {
			count, err = this.GetInt("goodsCount")
			if err != nil {
				log.Println("商品数量错误", err)
				return
			}
		} else {
			count, err = redis.Int(conn.Do("hget", "cart_"+strconv.Itoa(user.Id), skuid))
			if err != nil {
				log.Println("商品数量错误", err)
				this.Redirect("/user/cart", 302)
				return
			}
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
	// 需注意订单整个流程为事物操作，包括：
	// 1插入订单表记录，
	// 2插入订单商品表记录，
	//,3更新SKU表中的库存与销量
	// 一步撤销，步步撤销
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

		// 多用户并发改库存情况导致超卖问题，采用的处理机制是核验库存不一致就回滚，
		// 这样的操作限制过于严格，若库存十分充足，发生并发冲突，导致用户下单经常回滚，
		// 为提升用户体验，后台循环若干次自动代用户重新发起请求。
		// 这里限制循环次数：3，若3此仍失败，才返回并发库存错误
		//
		i := 3
		for i > 0 {
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
			// 获取此刻的库存，并保存
			preStock := goods.Stock
			// time.Sleep(5 * time.Second)
			log.Printf("当前用户：%d,当前记录的库存：%d", user.Id, preStock)

			//1 执行单个插入,初步完成订单创建
			orderGoods.Count = count
			orderGoods.Price = count * goods.Price
			o.Insert(&orderGoods)

			//2 更新库存，仅是这里更新没用，需要同步到数据库。
			goods.Stock -= count
			goods.Sales += count
			// 注意1和2再多用户并发操作时，导致超卖，故执行更新最新库存之前需要先查询库存跟之前取出来的是否一致
			updateCount, err := o.QueryTable("GoodsSKU").Filter("Id", goods.Id).Filter("Stock", preStock).Update(orm.Params{"Stock": goods.Stock, "Sales": goods.Sales})
			if err != nil {
				resp["code"] = 5
				resp["msg"] = "数据库查询商品信息失败"
				o.Rollback()
				log.Println("操作已回滚，信息如下：", err)
				log.Printf("用户id:%d,商品id:%d,库存数量：%d,所需数量：%d", user.Id, skuid, goods.Stock, count)
				this.Data["json"] = resp
				return
			}

			// 执行更新库存前，验证库存，发现库存已改变，则撤销所有操作
			if updateCount == 0 {
				if i > 0 {
					// 本次尝试失败且还有尝试机会
					i--
					log.Printf("用户id:%d,本次尝试失败,正在尝试第%d次机会", user.Id, 4-i)
					continue
				}
			} else {
				// 本次尝试成功，无需再次尝试
				log.Printf("本次尝试成功，终止尝试")
				break
			}
		}
		// 尝试结束，判断是用尽机会还是尝试成功
		if i == 0 {
			// 没有尝试机会了
			resp["code"] = 6
			resp["msg"] = "购买人数太多了，本次订单提交失败"
			o.Rollback()
			log.Println("操作已回滚：", err)
			this.Data["json"] = resp
			return
		}
		// 尝试成功了
		// 购物车中对应的商品删除
		conn.Do("hdel", "cart_"+strconv.Itoa(user.Id), goods.Id)
	}

	// 操作成功，返回成功信息
	resp["code"] = 200
	resp["msg"] = "OK"
	o.Commit()
	log.Println("订单创建成功！")
	this.Data["json"] = resp
	return
}

// 处理支付
func (this *OrderController) HandlePay() {
	var aliPublicKey = "" // 可选，支付宝提供给我们用于签名验证的公钥，通过支付宝管理后台获取
	var privateKey = `MIIEpgIBAAKCAQEA4JBmaOfYlR8l3CsyfwwlWRtA92k0sQczLNtMkXO24Iwos+Hb
	yPn8BH2ho2iWHJp+Tqw5l5l9au4J8NW+4DiOzWBmcFJkEpk3jOOfZNxpIj4vWbQw
	d2UKgYm1lc53kQtvUhCubZJHJlRS2lFfsSYB2QdL37ENy/enWMchsWHjUeEZjWco
	lpxRbaVnskKI+plGJ+v7dip6kjRRwRBZIPBa0cZOPW5vY9EzaYTaBsJNHgd9dFi4
	1DNvURRmhvPcGpD/Zns/PCJAwpPxPALs9fpc686VnAqEFIsaVnIFqHeUKXa6jWux
	fAjxFHpnWd/lkd9t3YDjt3Bi3uC89iEGqcVIZwIDAQABAoIBAQCMm320E+81t/IR
	wG52xFkiSQFNqO8YJUTywkFYFZcdVEUsFLB0T6pv+WXbFmJfeJC7m/TXqoCwEmnh
	BUTlyiQIDmM10zDbwFna+q9UDPo7OaqWRU/PglGouFwdd9C/3eQPA2jkLKImKshR
	8H+1QPIJPRtR7d+Qpfl/iffbxEn8eoQA8sJWp710Kl3SB5gStqrw0YYo8zIVsjLe
	BlsYfffLUztJCI38QTfjt7EGGh7kVO7cYhc9nTA6Eirf75vv6JuyzRTjqfZ3ktvq
	uXm/LXI1Pfa0qIZljYqtg97+E76d+rmHk6QcEN29q+D/gbkixdq2X+NHbCaWeGBA
	dqgesqWpAoGBAPglWjpN+T8B6E8AktxljvJtOtX8VoP/bzugjFMzrPMEfJ38n7wB
	SRoz37tUIqu6Cs7OOohHe+SidwKbdrL9dz4ig0D0IaaXvX7moiqOuqAlfPNnfgXs
	88txxANV2U4wvvKSlm1Rm+O3xnqOPbmW36K6uzVYL5zYlMXLGShsNjHTAoGBAOer
	+Kwe/N6NzWqqoLg6ShTMNpb1eW9aU+Pi6ItqIl7UOZhrjh5tddmIZWaUdYnuQov9
	24n+dlZNJz6WbELWnR+D7ylRCWduj8d5Va0+hbgnYkJ+0U9olCTOt20YpqqG0wWY
	qwITeXMPXy6lv5ORpf4NvPsBPCgsGLRw9zuBRh6dAoGBAMZFqjN+DBJxJrrBPZdG
	upIv/tvuFP7BQZKGNLliR+WhhyUBLmydJlj+a90VW+KU83/Mvm4XmAHWYns91vkr
	l3SZRQDIUH75LZtREvAoPSwq6Azge4ymiSHck/8KQGi+gEP4JqPQmlu4gql4MA+z
	Ypt20pDMFrcfQrhMEJ0A4cirAoGBAN0BuaC5jxHgxO3VCK23LaTZi9pHIymPSihD
	9wPIpDFC1A8Ly/BLC/oRnGpXhimnGeTir+Tc05dQ0vdqGK1Kf2npOuZ3YDlDx/XL
	UmiLFJWxPJOi15qhcXILogB5W8WiCP11vu2kFmAlce/WPwRQFcJe6MGrU/Ae4RKC
	Edi6YmIhAoGBALq8m+hFd0OcWbMbLLTn+6WZrNeMJM7UJlQ4ngOMq15TfwLvyaB8
	2gmayigvU+uOk1xlBOmtLivOCzlbt+heF4LId6jZyzlhZIQ/3xq4BrspocTyUwCX
	osEP+3wD4r5c0EolOVpmegh5LxXHSVLvu2Jid3gwg0YlXvunhC49hhQy
	` // 必须，上一步中使用 RSA签名验签工具 生成的私钥
	var appId = "2016101100656969"
	var client, err = alipay.New(appId, aliPublicKey, privateKey, false)

	// 将 key 的验证调整到初始化阶段
	if err != nil {
		log.Println(err)
		return
	}

	orderId := this.GetString("orderId")
	amount := this.GetString("amount")

	var p = alipay.TradePagePay{}
	p.NotifyURL = "http://xxx"
	p.ReturnURL = "http://192.168.123.174:8080/user/payok"
	p.Subject = "天天生鲜购物平台"
	p.OutTradeNo = orderId
	p.TotalAmount = amount
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"

	url, err := client.TradePagePay(p)
	if err != nil {
		log.Println(err)
	}

	var payURL = url.String()
	log.Println(payURL)
	// 这个 payURL 即是用于支付的 URL，可将输出的内容复制，到浏览器中访问该 URL 即可打开支付页面。
	this.Redirect(payURL, 302)
}

// 支付成功的状态处理（显示支付结果）
func (this *OrderController) PayOK() {
	this.TplName = "user_center_order.html"
	orderId := this.GetString("out_trade_no")
	if orderId == "" {
		log.Println("支付返回数据错误")
		this.Redirect("/user/userCenterOrder", 302)
		return
	}
	o := orm.NewOrm()
	_, err := o.QueryTable("OrderInfo").Filter("OrderId", orderId).Update(orm.Params{"Orderstatus": 0})
	if err != nil {
		log.Println("更新订单数据失败")
		this.Redirect("/user/userCenterOrder", 302)
		return
	}
	this.Redirect("/user/userCenterOrder", 302)
}
