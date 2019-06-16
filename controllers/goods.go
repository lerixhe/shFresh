package controllers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	"log"
	"shFresh/models"
	"shFresh/redispool"
	"strconv"
)

type GoodsController struct {
	beego.Controller
}

func (this *GoodsController) ShowIndex() {
	GetUser(&this.Controller)
	this.TplName = "index.html"
	//获取类型数据
	types := GetGoodsTypes(&this.Controller)
	//获取首页轮播图数据
	indexGoodsBanner := []models.IndexGoodsBanner{}
	o := orm.NewOrm()
	o.QueryTable("IndexGoodsBanner").OrderBy("Index").All(&indexGoodsBanner)
	this.Data["indexGoodsBanner"] = indexGoodsBanner
	// 获取促销商品数据
	promotionGoods := []models.IndexPromotionBanner{}
	o.QueryTable("IndexPromotionBanner").OrderBy("Index").All(&promotionGoods)
	this.Data["promotionGoods"] = promotionGoods
	// 首页展示商品数据
	// 创建一个切片goods，每个元素是一个容器：存储三种数据类型：商品类型(types)，文字商品(textgoods)，图片商品(imagegoods)
	//每种类型要记录类型，故使用map键值对存储。

	goods := make([]map[string]interface{}, len(types))
	for index, value := range types {
		temp := make(map[string]interface{})
		temp["type"] = value
		goods[index] = temp
		// 示例数据：goods[1]=map["type"]="新鲜水果"
	}
	//给每一个goods补充文字商品和图片商品
	for _, value := range goods {
		var textGoods []models.IndexTypeGoodsBanner
		var imageGoods []models.IndexTypeGoodsBanner
		//获取文字商品数据
		o.QueryTable("IndexTypeGoodsBanner").RelatedSel("GoodsType", "GoodsSKU").OrderBy("Index").Filter("GoodsType", value["type"]).Filter("DisplayType", 0).All(&textGoods)
		//获取图片商品数据
		o.QueryTable("IndexTypeGoodsBanner").RelatedSel("GoodsType", "GoodsSKU").OrderBy("Index").Filter("GoodsType", value["type"]).Filter("DisplayType", 1).All(&imageGoods)
		value["textGoods"] = textGoods
		value["imageGoods"] = imageGoods
	}
	// for i := 0; i < len(goods); i++ {
	// 	for k, y := range goods[i] {
	// 		log.Println("获取数据：key:", k, "value:", y)
	// 	}
	// }
	this.Data["goods"] = goods
}

//展示商品详情
func (this *GoodsController) ShowGoodsDetail() {
	unameBySession := GetUser(&this.Controller)
	GetGoodsTypes(&this.Controller)
	this.TplName = "detail.html"
	//根据商品sku的id获取全部sku和spu信息
	id, _ := this.GetInt("id")
	sku2goods := models.GoodsSKU{}
	o := orm.NewOrm()
	o.QueryTable("GoodsSKU").RelatedSel("Goods").Filter("Id", id).One(&sku2goods)
	this.Data["goodssku"] = sku2goods
	//展示新品推荐(2条)
	GetGoodsRecom(&this.Controller, sku2goods.GoodsType.Id, 2)
	// 添加用户历史记录
	//1. 判断用户是否登录
	if unameBySession == "" {
		log.Println("用户未登录，不记录浏览历史")
		return
	}
	//2.查询用户信息
	user := models.User{Name: unameBySession}
	o.Read(&user, "Name")
	//3.添加历史记录，使用redis存储
	conn := redispool.Redisclient.Get()
	defer conn.Close()
	//3.1把以前相同商品的历史记录删除
	conn.Do("lrem", "history_"+strconv.Itoa(user.Id), 0, id)
	//3.2把当前浏览的商品id存入key为history_xx的list中
	conn.Do("lpush", "history_"+strconv.Itoa(user.Id), id)

}
func (this *GoodsController) ShowGoodsList() {
	this.TplName = "list.html"
	typeId, err := this.GetInt("typeId")
	if err != nil {
		log.Println("传入的类型id错误：", err)
		return
	}
	sortId, err := this.GetInt("sortId")
	if err != nil {
		log.Println("传入的排序id错误：将按默认排序", err)
	}
	//展示登录信息
	GetUser(&this.Controller)
	GetGoodsTypes(&this.Controller)
	//展示新品推荐(2条)
	GetGoodsRecom(&this.Controller, typeId, 2)
	//查询该类型的所有SKU
	goods := make([]models.GoodsSKU, 1)
	o := orm.NewOrm()
	//.根据不同排序要求，查询数据.
	switch sortId {
	case 1:
		o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).OrderBy("Price").All(&goods)
		log.Println("已按价格排序")
	case 2:
		o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).OrderBy("Sales").All(&goods)
		log.Println("已按人气排序")
	default:
		o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).All(&goods)
		log.Println("已按默认排序")
		log.Println(sortId)
	}
	this.Data["goods"] = goods
	this.Data["typeId"] = typeId

}
func GetGoodsTypes(this *beego.Controller) (types []models.GoodsType) {
	o := orm.NewOrm()
	o.QueryTable("GoodsType").All(&types)
	this.Data["types"] = types
	log.Println("获取商品类型成功")
	return types
}

//新品推荐：获取同类型的两条最新商品数据
// 传入controller,类型id，个数，即可在前端展示新品推荐
func GetGoodsRecom(this *beego.Controller, typeId int, num int) {
	goodsNew := []models.GoodsSKU{}
	o := orm.NewOrm()
	o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).OrderBy("Time").Limit(num, 0).All(&goodsNew)
	this.Data["goodsNew"] = goodsNew
}
