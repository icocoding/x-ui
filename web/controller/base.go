package controller

import (
	"fmt"
	"net/http"
	"strings"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

type BaseController struct {
}

func IsFreeAuth(c *gin.Context) bool {
	free := strings.HasPrefix(c.Request.URL.Path, "/xui/subscription/link/")
	fmt.Printf("IsFreeAuth: %v %v\n", c.Request.URL.Path, free)
	return free
}

func (a *BaseController) checkLogin(c *gin.Context) {
	if IsFreeAuth(c) {
		c.Next()
	} else if !session.IsLogin(c) {
		if isAjax(c) {
			pureJsonMsg(c, false, "登录时效已过，请重新登录")
		} else {
			c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
		}
		c.Abort()
	} else {
		c.Next()
	}
}
