package controllers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	"log"
	"math"
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
	//获取所选分类
	typeId, err := this.GetInt("typeId")
	if err != nil {
		log.Println("传入的类型id错误：", err)
		return
	}
	//获取用户所选排序
	sortId, err := this.GetInt("sortId")
	if err != nil {
		log.Println("未获取到排序id：将按默认排序", err)
		sortId = 0
	}
	//获取用户所请求的页码
	pageIndex, err := this.GetInt("pageIndex")
	if err != nil {
		log.Println("未获取到页面，默认页码为1", err)
		pageIndex = 1
	}
	//回传所选信息
	this.Data["typeId"] = typeId
	this.Data["pageIndex"] = pageIndex
	this.Data["sortId"] = sortId

	//展示登录信息、商品分类
	GetUser(&this.Controller)
	GetGoodsTypes(&this.Controller)
	//展示新品推荐(2条)
	GetGoodsRecom(&this.Controller, typeId, 2)

	//分页实现
	//1.查询总记录数
	o := orm.NewOrm()
	count, _ := o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).Count()
	//2.设定单页记录数
	pageSize := 5
	//3.得出所需总页数
	pageCount := int(math.Ceil(float64(count) / float64(pageSize)))
	//3.1校验请求的页码与总页数
	if pageCount < pageIndex {
		pageIndex = pageCount
	}
	// 4.根据总页数和用户选择的页码，创建页面显示的页码
	pages := PageTool(pageCount, pageIndex)
	this.Data["pages"] = pages
	//5.根据分页，设置数据库查询的开始位置
	start := (pageIndex - 1) * pageSize
	//获取上一页页码
	prePage := pageIndex - 1
	if prePage <= 1 {
		prePage = 1
	}
	this.Data["prePage"] = prePage
	//获取下一页页码
	nextPage := pageIndex + 1
	if nextPage > pageCount {
		nextPage = pageCount
	}
	this.Data["nextPage"] = nextPage

	//查询该类型的所有SKU
	goods := make([]models.GoodsSKU, 1)
	//1.根据不同排序要求，查询数据.
	switch sortId {
	case 1:
		o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).OrderBy("Price").Limit(pageSize, start).All(&goods)
		log.Println("已按价格排序")
	case 2:
		o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).OrderBy("Sales").Limit(pageSize, start).All(&goods)
		log.Println("已按人气排序")
	default:
		o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).Limit(pageSize, start).All(&goods)
		log.Println("已按默认排序")
		log.Println(sortId)
	}
	this.Data["goods"] = goods

}

//显示搜索结果
func (this *GoodsController) ShowSearch() {
	this.TplName = "search.html"
	//展示登录信息、商品分类
	GetGoodsTypes(&this.Controller)
	GetUser(&this.Controller)

	//获取用户所选排序
	sortId, err := this.GetInt("sortId")
	if err != nil {
		log.Println("未获取到排序id：将按默认排序", err)
		sortId = 0
	}
	//获取用户所请求的页码
	pageIndex, err := this.GetInt("pageIndex")
	if err != nil {
		log.Println("未获取到页面，默认页码为1", err)
		pageIndex = 1
	}

	keywords := this.GetString("keywords")
	//回传所选信息
	this.Data["pageIndex"] = pageIndex
	this.Data["sortId"] = sortId
	this.Data["keywords"] = keywords

	log.Println("正在匹配关键词：", keywords)
	goods := []models.GoodsSKU{}
	o := orm.NewOrm()

	//分页实现
	//1.查询总记录数
	count, _ := o.QueryTable("GoodsSKU").Filter("Name__icontains", keywords).Count()
	//2.设定单页记录数
	pageSize := 10
	//3.得出所需总页数
	pageCount := int(math.Ceil(float64(count) / float64(pageSize)))
	//3.1校验请求的页码与总页数
	if pageCount < pageIndex {
		pageIndex = pageCount
	}
	// 4.根据总页数和用户选择的页码，创建页面显示的页码
	pages := PageTool(pageCount, pageIndex)
	this.Data["pages"] = pages
	//5.根据分页，设置数据库查询的开始位置
	if pageCount < pageIndex {
		pageIndex = pageCount
	}
	start := (pageIndex - 1) * pageSize
	//获取上一页页码
	prePage := pageIndex - 1
	if prePage <= 1 {
		prePage = 1
	}
	this.Data["prePage"] = prePage
	//获取下一页页码
	nextPage := pageIndex + 1
	if nextPage > pageCount {
		nextPage = pageCount
	}
	this.Data["nextPage"] = nextPage

	//查询该类型的所有SKU
	//1.根据不同排序要求，查询数据.
	//校验容错处理

	switch sortId {
	case 1:
		o.QueryTable("GoodsSKU").Filter("Name__icontains", keywords).OrderBy("Price").Limit(pageSize, start).All(&goods)
		log.Println("已按价格排序")
	case 2:
		o.QueryTable("GoodsSKU").Filter("Name__icontains", keywords).OrderBy("Sales").Limit(pageSize, start).All(&goods)
		log.Println("已按人气排序")
	default:
		o.QueryTable("GoodsSKU").Filter("Name__icontains", keywords).Limit(pageSize, start).All(&goods)
		log.Println("已按默认排序")
		log.Println("排序标识：", sortId)
	}
	this.Data["goods"] = goods

}

//获取商品类型：获取所有的商品类型，并输出到页面
func GetGoodsTypes(this *beego.Controller) (types []models.GoodsType) {
	o := orm.NewOrm()
	o.QueryTable("GoodsType").All(&types)
	this.Data["types"] = types
	log.Println("获取商品类型成功")
	return types
}

//分页助手：使用传递进来的总页数和目标页面，进行合理的分页展示
func PageTool(pageCount, pageIndex int) (pages []int) {
	log.Println("总页数", pageCount)

	if pageCount <= 5 {
		pages = make([]int, pageCount)
		for i := range pages {
			pages[i] = i + 1
		}
	} else if pageIndex <= 3 {
		pages = []int{1, 2, 3, 4, 5}
	} else if pageIndex >= pageCount-4 {
		pages = []int{pageCount - 4, pageCount - 3, pageCount - 2, pageCount - 1, pageCount}
	} else {
		pages = []int{pageIndex - 2, pageIndex - 1, pageIndex, pageIndex + 1, pageIndex + 2}
	}
	return
}

//新品推荐：获取同类型的两条最新商品数据
// 传入controller,类型id，个数，即可在前端展示新品推荐
func GetGoodsRecom(this *beego.Controller, typeId int, num int) {
	goodsNew := []models.GoodsSKU{}
	o := orm.NewOrm()
	o.QueryTable("GoodsSKU").RelatedSel("GoodsType").Filter("GoodsType__Id", typeId).OrderBy("Time").Limit(num, 0).All(&goodsNew)
	this.Data["goodsNew"] = goodsNew
}
