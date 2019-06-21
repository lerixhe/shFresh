package routers

import (
	"github.com/astaxie/beego/context"
	"log"
	"shFresh/controllers"

	"github.com/astaxie/beego"
)

func init() {
	//插入过滤器 : 过滤用户控制器路由；再找到路由之后，执行之前；判断用户登录状态
	beego.InsertFilter("/user/*", beego.BeforeExec, filterFunc)
	//注册用户
	beego.Router("/register", &controllers.UserController{}, "get:ShowReg;post:HandleReg")
	//激活用户
	beego.Router("/active", &controllers.UserController{}, "get:ActiveUser")
	//用户登录
	beego.Router("/login", &controllers.UserController{}, "get:ShowLogin;post:HandleLogin")
	//商城首页
	beego.Router("/index", &controllers.GoodsController{}, "get:ShowIndex")
	beego.Router("/", &controllers.GoodsController{}, "get:ShowIndex")
	//用户退出
	beego.Router("/user/logout", &controllers.UserController{}, "get:HandleLogout")
	//显示用户中心 : 用户信息
	beego.Router("/user/userCenterInfo", &controllers.UserController{}, "get:ShowUserInfo")
	//显示用户中心：用户订单
	beego.Router("/user/userCenterOrder", &controllers.UserController{}, "get:ShowUserOrder")
	//显示用户中心：用户地址
	beego.Router("/user/userCenterSite", &controllers.UserController{}, "get:ShowUserSite;post:HandleUserSite")
	// 显示商品详情
	beego.Router("/goodsDetail", &controllers.GoodsController{}, "get:ShowGoodsDetail")
	//显示商品列表
	beego.Router("/goodsList", &controllers.GoodsController{}, "get:ShowGoodsList")
	// 显示搜索结果
	beego.Router("/goodsSearch", &controllers.GoodsController{}, "get:ShowSearch")
	// 添加到购物车
	beego.Router("/user/addCart", &controllers.CartController{}, "post:HandleAddCart")
	// 显示购物车
	beego.Router("/user/cart", &controllers.CartController{}, "get:ShowCart")
	// 更新购物车
	beego.Router("/user/UpdateCart", &controllers.CartController{}, "post:HandleUpdateCart")
	// 删除购物车商品
	beego.Router("/user/deleteCart", &controllers.CartController{}, "post:DeleteCart")
	// 显示订单页面
	beego.Router("/user/showOrder", &controllers.OrderController{}, "post:ShowOrder")
	// 创建订单
	beego.Router("/user/addOrder", &controllers.OrderController{}, "post:AddOrder")
	// 接受处理支付的url
	beego.Router("/user/pay", &controllers.OrderController{}, "get:HandlePay")
	// 支付成功
	beego.Router("user/payok", &controllers.OrderController{}, "get:PayOK")
}

//全局变量
var filterFunc = func(ctx *context.Context) {
	userName := ctx.Input.Session("userName")
	log.Println("当前用户session信息:", userName)
	if userName == nil {
		ctx.Redirect(302, "/login")
		return
	}
}
