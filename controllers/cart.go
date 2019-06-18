package controllers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	"github.com/gomodule/redigo/redis"
	"log"
	"shFresh/models"
	"shFresh/redispool"
	"strconv"
)

type CartController struct {
	beego.Controller
}

func (this *CartController) ShowCart() {
	this.TplName = "cart.html"
	userName := GetUser(&this.Controller)
	user := models.User{Name: userName}
	o := orm.NewOrm()
	o.Read(&user, "Name")
	conn := redispool.Redisclient.Get()
	defer conn.Close()
	goodsMap, err := redis.IntMap(conn.Do("hgetall", "cart_"+strconv.Itoa(user.Id))) //map[string]int
	if err != nil {
		log.Println("获取购物车商品列表失败")
	}
	// 创建一个容器，存储商品列表。
	// 将redis中的map[string]int 转为[]map[string]interface{}
	goods := []map[string]interface{}{}
	for k, v := range goodsMap {
		id, _ := strconv.Atoi(k)
		goodsSKU := models.GoodsSKU{Id: id}
		o.Read(&goodsSKU)
		temp := make(map[string]interface{})
		temp["goodssku"] = goodsSKU
		temp["count"] = v
		goods = append(goods, temp)
	}
	this.Data["goods"] = goods
}

//接受并处理【加入购物车】产生的post请求
func (this *CartController) HandleAddCart() {
	//准备构建一个json，作为回应
	resp := make(map[string]interface{})
	defer this.ServeJSON()
	//虽然这里也是才登录过滤器中的url，然而这里是post请求，并不能自动跳转
	// 登录状态处理
	userName := this.GetSession("userName")
	if userName == nil {
		resp["code"] = 1
		resp["msg"] = "用户未登录"
		this.Data["json"] = resp
		return
	}
	skuid, err := this.GetInt("skuid")
	count, err := this.GetInt("count")
	if err != nil {
		resp["code"] = 2
		resp["msg"] = "请求的数据不存在:"
		this.Data["json"] = resp
		return
	}
	log.Println("获取到商品id和数量：", skuid, count)
	//查询一些需要用到的数据
	o := orm.NewOrm()
	user := models.User{Name: userName.(string)}
	o.Read(&user, "Name")
	//处理数据：将购物车数据(商品id，数量)存入redis。如果购物车已有某个商品id，直接数量+1.没有的话加入商品id和数量1
	conn := redispool.Redisclient.Get()
	defer conn.Close()
	// 1.先从redis获取购物车内容
	preCount, _ := redis.Int(conn.Do("hget", "cart_"+strconv.Itoa(user.Id), skuid))
	conn.Do("hset", "cart_"+strconv.Itoa(user.Id), skuid, count+preCount)
	// 获取redis中所有购物商品数量
	cartCount := GetCartCount(&this.Controller)
	resp["code"] = 5
	resp["msg"] = "ok"
	resp["cartCount"] = cartCount
	this.Data["json"] = resp

}

// 获取购物车中商品数量
//返回用户信息和购物车数据
func GetCartCount(this *beego.Controller) (cartCount int) {
	userName := this.GetSession("userName")
	if userName == nil {
		this.Data["cartCount"] = 0
		log.Println("用户未登录,购物车数量默认为0")
		return 0
	}
	user := models.User{Name: userName.(string)}
	o := orm.NewOrm()
	err := o.Read(&user, "Name")
	if err != nil {
		log.Println(err, user)
	}
	conn := redispool.Redisclient.Get()
	defer conn.Close()
	re, err := conn.Do("hlen", "cart_"+strconv.Itoa(user.Id))
	cartCount, _ = redis.Int(re, err)
	this.Data["cartCount"] = cartCount
	log.Println("获取购物车商品数量成功：", cartCount)
	return
}
