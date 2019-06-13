package controllers

import (
	"encoding/base64"
	"log"
	"regexp"
	"shFresh/models"
	"strconv"

	"github.com/astaxie/beego/utils"

	"github.com/astaxie/beego/orm"

	"github.com/astaxie/beego"
)

/*
   控制器四部曲：
   1.获取数据
   2.校验数据
   3.处理数据
   4.返回视图
   路由四部曲：
   1.
*/
// 用户模块控制器
type UserController struct {
	beego.Controller
}

//显示注册页面
func (this *UserController) ShowReg() {
	this.TplName = "register.html"
}

//处理注册数据
func (this *UserController) HandleReg() {
	userName := this.GetString("user_name")
	pwd := this.GetString("pwd")
	cpwd := this.GetString("cpwd")
	email := this.GetString("email")
	//校验数据格式
	if userName == "" || pwd == "" || cpwd == "" || email == "" {
		this.TplName = "register.html"
		this.Data["errmsg"] = "数据不完整，请检查！"
		return
	}
	if pwd != cpwd {
		this.TplName = "register.html"
		this.Data["errmsg"] = "两次密码输入不一致，请重新填写！"
		return
	}
	//不能使用原生字符串，负责识别不出一些转义字符
	expr := "^[A-Za-z0-9\u4e00-\u9fa5]+@[a-zA-Z0-9_-]+(\\.[a-zA-Z0-9_-]+)+$"
	reg, err := regexp.Compile(expr)
	if err != nil {
		log.Println("regexpstring err", err)
		return
	}
	if reg.FindString(email) == "" {
		this.TplName = "register.html"
		this.Data["errmsg"] = "邮箱格式不正确，请重新输入"
		return
	}
	//写入数据库，用户表
	o := orm.NewOrm()
	user := models.User{
		Name:     userName,
		PassWord: pwd,
		Email:    email,
	}
	_, err = o.Insert(&user)
	if err != nil {
		this.TplName = "register.html"
		this.Data["errmsg"] = "注册失败，请尝试换个用户名注册！"
		return
	}
	//发送邮件
	emailConfig := `{
        "username":"185734549@qq.com",
        "password":"jkapqqylhhizbidf",
        "host":"smtp.qq.com",
        "port":587}`
	emailConn := utils.NewEMail(emailConfig)
	emailConn.From = "185734549@qq.com"
	emailConn.To = []string{email}
	emailConn.Subject = "天天生鲜用户激活"
	emailConn.Text = "http://127.0.0.1:8080/active?id=" + strconv.Itoa(user.Id)
	err = emailConn.Send()
	if err != nil {
		log.Println("email err:", err, emailConn.From, emailConn.To)
		this.TplName = "register.html"
		this.Data["errmsg"] = "系统错误,请稍后重试"
		return
	}
	//返回视图
	this.Ctx.WriteString("注册成功，已向您的注册邮箱发送激活链接,请登录邮箱点击进行激活！")
}

//处理激活请求
func (this *UserController) ActiveUser() {
	id, err := this.GetInt("id")
	if err != nil {
		this.Data["errmsg"] = "要激活的用户不存在！"
		this.TplName = "register.html"
		return
	}
	o := orm.NewOrm()
	user := models.User{Id: id}
	err = o.Read(&user)
	if err != nil {
		this.Data["errmsg"] = "要激活的用户不存在！"
		this.TplName = "register.html"
		return
	}
	user.Active = true
	o.Update(&user)
	//返回视图
	this.Redirect("/login", 302)

}

//显示登录页面
func (this *UserController) ShowLogin() {
	userNameBase64 := this.Ctx.GetCookie("userName")
	temp, err := base64.StdEncoding.DecodeString(userNameBase64)
	if err != nil {
		log.Println("base64转换错误！")
		this.TplName = "login.html"
		return
	}

	if string(temp) == "" {
		this.Data["checked"] = ""
	} else {
		this.Data["checked"] = "checked"
	}
	this.Data["username"] = string(temp)
	this.TplName = "login.html"
}

//处理登录
func (this *UserController) HandleLogin() {
	//获得登录数据
	userName := this.GetString("username")
	pwd := this.GetString("pwd")
	log.Println("username:", userName, "pwd:", pwd)
	//校验数据
	if userName == "" || pwd == "" {
		this.Data["errmsg"] = "登录信息不完整！"
		this.TplName = "login.html"
		return
	}
	//写入数据库
	o := orm.NewOrm()
	user := models.User{Name: userName}
	err := o.Read(&user, "Name") //非主键字段需要特别指明
	log.Println(user)
	if err != nil || user.PassWord != pwd {
		this.Data["errmsg"] = "用户名或密码错误，请重试"
		this.TplName = "login.html"
		return
	}
	log.Println(user.Active)
	if user.Active == false {
		this.Data["errmsg"] = "用户未激活，请前往邮箱激活！"
		this.TplName = "login.html"
		return
	}
	remember := this.GetString("remember")
	//保存用户名到cookie,由于cookie不能写中文，使用base64编码
	temp := base64.StdEncoding.EncodeToString([]byte(userName))
	if remember == "on" {
		this.Ctx.SetCookie("userName", temp, 24*60*60*30) //30天有效
	} else {
		this.Ctx.SetCookie("userName", temp, -1)
	}
	log.Println("cookies:", this.Ctx.GetCookie("userName"))
	//登录成功设置session
	this.SetSession("userName", userName)
	//返回视图
	//this.Ctx.WriteString("登录成功！")
	this.Ctx.Redirect(302, "/")

}

//显示首页
func (this *UserController) ShowIndex() {
	//登录判断
	//思路：开启session存储登录信息，使用路由过滤器控制访问权限

	this.TplName = "index.html"
}

//退出登录
func (this *UserController) HandleLogout() {
	this.DelSession("userName")
	this.Redirect("/login", 302)
}

//显示用户中心：用户信息
func (this *UserController) ShowUserInfo() {
	userName := this.GetSession("userName")
	this.Data["userName"] = userName.(string)
	this.Data["infoActive"] = "active"
	this.TplName = "user_center_info.html"
}

//显示用户中心：用户订单
func (this *UserController) ShowUserOrder() {
	userName := this.GetSession("userName")
	this.Data["userName"] = userName.(string)
	this.Data["orderActive"] = "active"
	this.TplName = "user_center_order.html"
}

//显示用户中心：用户订单
func (this *UserController) ShowUserSite() {
	userName := this.GetSession("userName")
	this.Data["userName"] = userName.(string)
	this.Data["siteActive"] = "active"
	this.TplName = "user_center_site.html"
}
