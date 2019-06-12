package routers

import (
	"shFresh/controllers"

	"github.com/astaxie/beego"
)

func init() {
	beego.Router("/", &controllers.MainController{})
	beego.Router("/register", &controllers.UserController{}, "get:ShowReg;post:HandleReg")
	//激活用户
	beego.Router("/active", &controllers.UserController{}, "get:ActiveUser")
	//用户登录
	beego.Router("/login", &controllers.UserController{}, "get:ShowLogin;post:HandleLogin")
}
