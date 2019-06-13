package controllers

import (
	"github.com/astaxie/beego"
)

type GoodsController struct {
	beego.Controller
}

func (this *GoodsController) ShowIndex() {
	userName := this.GetSession("userName")
	if userName == nil {
		//未登录
		this.Data["userName"] = ""
	} else {
		//已登录
		this.Data["userName"] = userName.(string)
	}
	this.TplName = "index.html"
}
