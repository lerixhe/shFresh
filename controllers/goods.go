package controllers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	"shFresh/models"
)

type GoodsController struct {
	beego.Controller
}

func (this *GoodsController) ShowIndex() {
	GetUser(&this.Controller)
	this.TplName = "index.html"
	//获取类型数据
	types := []models.GoodsType{}
	o := orm.NewOrm()
	o.QueryTable("GoodsType").All(&types)
	this.Data["types"] = types
	//获取首页轮播图数据
	indexGoodsBanner := []models.IndexGoodsBanner{}
	o.QueryTable("IndexGoodsBanner").OrderBy("Index").All(&indexGoodsBanner)
	this.Data["indexGoodsBanner"] = indexGoodsBanner
	// 获取促销商品数据
	promotionGoods := []models.IndexPromotionBanner{}
	o.QueryTable("IndexPromotionBanner").OrderBy("Index").All(&promotionGoods)
	this.Data["promotionGoods"] = promotionGoods
	// 首页展示商品数据
	// 创建一个容器，长度为类型个数，存储类型为商品类型和商品SKU
	// goods：=make([]interface{},len(types))
	// for index,value:=range types{
	// 	goods[index]=value
	// }
}
