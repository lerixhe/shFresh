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

	//新品推荐：获取同类型的两条最新商品数据
	goodsNew := []models.GoodsSKU{}
	o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", sku2goods.GoodsType.Id).OrderBy("Time").Limit(2, 0).All(&goodsNew)
	this.Data["goodsNew"] = goodsNew

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
func GetGoodsTypes(this *beego.Controller) (types []models.GoodsType) {
	o := orm.NewOrm()
	o.QueryTable("GoodsType").All(&types)
	this.Data["types"] = types
	log.Println("获取类型成功：", types)
	return types
}
