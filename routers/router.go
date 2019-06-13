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
	beego.Router("/", &controllers.GoodsController{}, "get:ShowIndex")
	//用户退出
	beego.Router("/user/logout", &controllers.UserController{}, "get:HandleLogout")
	//显示用户中心 : 用户信息
	beego.Router("/user/userCenterInfo", &controllers.UserController{}, "get:ShowUserInfo")
	//显示用户中心：用户订单
	beego.Router("/user/userCenterOrder", &controllers.UserController{}, "get:ShowUserOrder")
	//显示用户中心：用户地址
	beego.Router("/user/userCenterSite", &controllers.UserController{}, "get:ShowUserSite;post:HandleUserSite")
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
