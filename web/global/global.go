package global

import (
	"context"
	_ "unsafe"

	"github.com/robfig/cron/v3"
)

var webServer WebServer

type WebServer interface {
	GetCron() *cron.Cron
	GetCtx() context.Context
	ReadAssets(path string) ([]byte, error) // open file
}

func SetWebServer(s WebServer) {
	webServer = s
}

func GetWebServer() WebServer {
	return webServer
}
