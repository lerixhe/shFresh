package main

import (
	_ "shFresh/models"
	_ "shFresh/routers"

	"github.com/astaxie/beego"
)

func main() {
	beego.Run()
}
