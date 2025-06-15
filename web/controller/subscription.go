package controller

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"x-ui/database/model"
	"x-ui/web/global"
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"

	"github.com/dop251/goja"
)

type SubscriptionController struct {
	subscriptionService service.SubscriptionService
	inboundService      service.InboundService
	xrayService         service.XrayService
	settingService      service.SettingService
	vm                  *goja.Runtime
}

func NewSubscriptionController(g *gin.RouterGroup) *SubscriptionController {
	a := &SubscriptionController{}
	a.initRouter(g)
	// a.startTask()

	a.vm = goja.New()
	// 读取 JavaScript 文件内容 assets
	jsXray := "js/model/xray.js"
	jsModels := "js/model/models.js"
	jsUtils := "js/util/utils.js"
	jsLinkUtil := "js/util/link-util.js"
	jsBase64 := "base64/base64.min.js"

	err := a.initJs(jsUtils)
	if err != nil {
		fmt.Printf("执行 jsUtils 文件失败: %v \n", err)
		return nil
	}
	err = a.initJs(jsBase64)
	if err != nil {
		fmt.Printf("执行 jsBase64 文件失败: %v \n", err)
		return nil
	}
	err = a.initJs(jsXray)
	if err != nil {
		fmt.Printf("执行 jsXray 文件失败: %v \n", err)
		return nil
	}
	err = a.initJs(jsModels)
	if err != nil {
		fmt.Printf("执行 jsModels 文件失败: %v \n", err)
		return nil
	}
	err = a.initJs(jsLinkUtil)
	if err != nil {
		fmt.Printf("执行 jsLinkUtil 文件失败: %v \n", err)
		return nil
	}
	return a
}

func (a *SubscriptionController) initJs(jsPath string) error {
	webServer := global.GetWebServer()
	jsContent, err := webServer.ReadAssets(jsPath)
	// var jsContent []byte
	// _, err = file.Read(jsContent)
	if err != nil {
		fmt.Printf("读取 %s 文件失败: %v\n", jsPath, err)
		return nil
	}
	// 在 JavaScript 虚拟机中执行代码
	_, err = a.vm.RunString(string(jsContent))
	if err != nil {
		fmt.Printf("执行 %s 文件失败: %v \n", jsPath, err)
		return nil
	}
	return nil
}
func (a *SubscriptionController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/subscription")

	g.POST("/list", a.getSubscriptions)
	g.POST("/add", a.addSubscription)
	g.POST("/del/:id", a.delSubscription)
	g.POST("/upd/:id", a.updSubscription)
	g.GET("/link/:token", a.getSubscriptionByToken)
}
func (a *SubscriptionController) setOutput(c *gin.Context, status int, data []string) {
	var output string
	if len(data) > 0 {
		output = strings.Join(data, "\r\n")
	}
	raw := c.Query("raw")
	c.Status(status)
	if raw != "1" {
		output = base64.StdEncoding.EncodeToString([]byte(output))
	}
	c.Writer.WriteString(output)
}
func (a *SubscriptionController) getSubscriptionByToken(c *gin.Context) {
	token := c.Param("token")
	subList := make([]string, 0)

	subscription, err := a.subscriptionService.GetSubscriptionByToken(token)
	if err != nil && subscription == nil {
		a.setOutput(c, 401, subList)
		return
	}
	fmt.Printf("订阅 %d 获取成功\n", subscription.Id)
	// 判断有效期
	if !subscription.Enable ||
		(subscription.ExpiryTime > 0 &&
			subscription.ExpiryTime < time.Now().Unix()*1000) {
		fmt.Printf("订阅已禁用or已过期\n")
		a.setOutput(c, 403, subList)
		return
	}

	// 是否自动更新
	autoUpdate := subscription.AutoUpdate
	if autoUpdate {

		// 创建新的节点
		inboundIds, err := a.subscriptionService.AutoUpdateSubscription(subscription)
		if err != nil {
			fmt.Printf("创建新的节点 error: %v", err)
			a.setOutput(c, 500, subList)
			return
		}
		idsStr, err := json.Marshal(inboundIds)
		if err != nil {
			fmt.Printf("创建新的节点 error: %v", err)
			a.setOutput(c, 500, subList)
			return
		}
		subscription.InboundIds = string(idsStr)

		a.xrayService.SetToNeedRestart()

	}

	inboundIds := subscription.InboundIds
	// 开始获取入站节点
	var ids []int
	err = json.Unmarshal([]byte(inboundIds), &ids)
	if err != nil {
		fmt.Printf("json.Unmarshal error: %v", err)
		a.setOutput(c, 500, subList)
		return
	}
	inbounds, err := a.inboundService.GetInboundsByIds(ids)
	if err != nil {
		fmt.Printf("获取订阅 inbounds error: %v", err)
		a.setOutput(c, 500, subList)
		return
	}
	allSetting, _ := a.settingService.GetAllSetting()

	allSettingJson, err := json.Marshal(allSetting)
	if err != nil {
		fmt.Printf("Marshal allSetting error: %v", err)
		return
	}

	for _, inbound := range inbounds {
		param, err := json.Marshal(inbound)
		if err != nil {
			fmt.Printf("Marshal error: %v", err)
			continue
		}
		link, err := a.vm.RunString(fmt.Sprintf("genLink(%s, %s)", string(param), string(allSettingJson)))
		if err != nil {
			fmt.Printf("genLink error: %v", err)
			continue
		}
		subList = append(subList, link.String())
	}
	a.setOutput(c, 200, subList)
}

func (a *SubscriptionController) getSubscriptions(c *gin.Context) {
	user := session.GetLoginUser(c)
	subscriptions, err := a.subscriptionService.GetSubscriptions(user.Id)
	if err != nil {
		jsonMsg(c, "获取", err)
		return
	}
	jsonObj(c, subscriptions, nil)
}

func genToken() string {
	nanoseconds := time.Now().UnixNano()

	// 将纳秒时间戳转换为字符串
	timestampStr := fmt.Sprintf("%d", nanoseconds)

	// 计算字符串的 MD5 值
	md5Hash := md5.Sum([]byte(timestampStr))

	// 将 MD5 值转换为字符串
	md5Str := hex.EncodeToString(md5Hash[:])

	return md5Str
}

func (a *SubscriptionController) addSubscription(c *gin.Context) {
	subscription := &model.Subscription{}
	err := c.ShouldBind(subscription)
	if err != nil {
		jsonMsg(c, "添加", err)
		return
	}
	user := session.GetLoginUser(c)
	subscription.UserId = user.Id
	subscription.Token = genToken()
	err = a.subscriptionService.AddSubscription(subscription)
	jsonMsg(c, "添加", err)
	if err == nil {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *SubscriptionController) delSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "删除", err)
		return
	}
	err = a.subscriptionService.DelSubscription(id)
	jsonMsg(c, "删除", err)
	if err == nil {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *SubscriptionController) updSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "修改", err)
		return
	}
	subscription := &model.Subscription{
		Id: id,
	}
	err = c.ShouldBind(subscription)
	if err != nil {
		jsonMsg(c, "修改", err)
		return
	}
	err = a.subscriptionService.UpdateSubscription(subscription)
	jsonMsg(c, "修改", err)
	if err == nil {
		a.xrayService.SetToNeedRestart()
	}
}
