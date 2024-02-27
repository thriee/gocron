package routers

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-macaron/binding"
	"github.com/go-macaron/gzip"
	"github.com/go-macaron/toolbox"
	"github.com/rakyll/statik/fs"
	"github.com/thriee/gocron/internal/dto"
	"github.com/thriee/gocron/internal/handler"
	"github.com/thriee/gocron/internal/pkg/app"
	"github.com/thriee/gocron/internal/pkg/logger"
	"github.com/thriee/gocron/internal/pkg/utils"
	"gopkg.in/macaron.v1"

	_ "github.com/thriee/gocron/internal/statik"
)

const (
	urlPrefix = "/api"
	staticDir = "public"
)

var statikFS http.FileSystem

func init() {
	var err error
	statikFS, err = fs.New()
	if err != nil {
		log.Fatal(err)
	}
}

// Register 路由注册
func Register(m *macaron.Macaron) {
	m.SetURLPrefix(urlPrefix)
	// 所有GET方法，自动注册HEAD方法
	m.SetAutoHead(true)
	m.Get("/", func(ctx *macaron.Context) {
		file, err := statikFS.Open("/index.html")
		if err != nil {
			logger.Error("读取首页文件失败: %s", err)
			ctx.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = io.Copy(ctx.Resp, file)

	})

	installHandler := new(handler.Install)

	// 系统安装
	m.Group("/install", func() {
		m.Post("/store", binding.Bind(dto.InstallForm{}), installHandler.Store)
		m.Get("/status", func(ctx *macaron.Context) string {
			jsonResp := utils.JsonResponse{}
			return jsonResp.Success("", app.Installed)
		})
	})

	userHandler := new(handler.User)

	// 用户
	m.Group("/user", func() {
		m.Get("", userHandler.Index)
		m.Get("/:id", userHandler.Detail)
		m.Post("/store", binding.Bind(dto.UserForm{}), userHandler.Store)
		m.Post("/remove/:id", userHandler.Remove)
		m.Post("/login", userHandler.ValidateLogin)
		m.Post("/enable/:id", userHandler.Enable)
		m.Post("/disable/:id", userHandler.Disable)
		m.Post("/editMyPassword", userHandler.UpdateMyPassword)
		m.Post("/editPassword/:id", userHandler.UpdatePassword)
	})

	taskHandler := new(handler.Task)
	taskLogHandler := new(handler.TaskLog)

	// 定时任务
	m.Group("/task", func() {
		m.Post("/store", binding.Bind(dto.TaskForm{}), taskHandler.Store)
		m.Get("/:id", taskHandler.Detail)
		m.Get("", taskHandler.Index)
		m.Post("/remove/:id", taskHandler.Remove)
		m.Post("/enable/:id", taskHandler.Enable)
		m.Post("/disable/:id", taskHandler.Disable)
		m.Get("/run/:id", taskHandler.Run)

		m.Get("/log", taskLogHandler.Index)
		m.Post("/log/clear", taskLogHandler.Clear)
		m.Post("/log/stop", taskLogHandler.Stop)
	})

	hostHandler := new(handler.Host)

	// 主机
	m.Group("/host", func() {
		m.Get("/:id", hostHandler.Detail)
		m.Get("", hostHandler.Index)
		m.Get("/all", hostHandler.All)

		m.Get("/ping/:id", hostHandler.Ping)
		m.Post("/remove/:id", hostHandler.Remove)
		m.Post("/store", binding.Bind(dto.HostForm{}), hostHandler.Store)
	})

	loginLogHandler := new(handler.LoginLog)
	manageHandler := new(handler.Manage)
	// 管理
	m.Group("/system", func() {
		m.Group("/slack", func() {
			m.Get("", manageHandler.Slack)
			m.Post("/update", manageHandler.UpdateSlack)
			m.Post("/channel", manageHandler.CreateSlackChannel)
			m.Post("/channel/remove/:id", manageHandler.RemoveSlackChannel)
		})
		m.Group("/mail", func() {
			m.Get("", manageHandler.Mail)
			m.Post("/update", binding.Bind(dto.MailServerForm{}), manageHandler.UpdateMail)
			m.Post("/user", manageHandler.CreateMailUser)
			m.Post("/user/remove/:id", manageHandler.RemoveMailUser)
		})
		m.Group("/webhook", func() {
			m.Get("", manageHandler.WebHook)
			m.Post("/update", manageHandler.UpdateWebHook)
		})
		m.Get("/login-log", loginLogHandler.Index)
	})

	// API
	m.Group("/v1", func() {
		m.Post("/tasklog/remove/:id", taskLogHandler.Remove)
		m.Post("/task/enable/:id", taskHandler.Enable)
		m.Post("/task/disable/:id", taskHandler.Disable)
	}, apiAuth)

	// 404错误
	m.NotFound(func(ctx *macaron.Context) string {
		jsonResp := utils.JsonResponse{}

		return jsonResp.Failure(utils.NotFound, "您访问的页面不存在")
	})
	// 50x错误
	m.InternalServerError(func(ctx *macaron.Context) string {
		jsonResp := utils.JsonResponse{}

		return jsonResp.Failure(utils.ServerError, "服务器内部错误, 请稍后再试")
	})
}

// RegisterMiddleware 中间件注册
func RegisterMiddleware(m *macaron.Macaron) {
	m.Use(macaron.Logger())
	m.Use(macaron.Recovery())
	if macaron.Env != macaron.DEV {
		m.Use(gzip.Gziper())
	}
	m.Use(
		macaron.Static(
			"",
			macaron.StaticOptions{
				Prefix:     staticDir,
				FileSystem: statikFS,
			},
		),
	)
	if macaron.Env == macaron.DEV {
		m.Use(toolbox.Toolboxer(m))
	}
	m.Use(macaron.Renderer())
	m.Use(checkAppInstall)
	m.Use(ipAuth)
	m.Use(userAuth)
	m.Use(urlAuth)
}

// region 自定义中间件

/** 检测应用是否已安装 **/
func checkAppInstall(ctx *macaron.Context) {
	if app.Installed {
		return
	}
	if strings.HasPrefix(ctx.Req.URL.Path, "/install") || ctx.Req.URL.Path == "/" {
		return
	}
	jsonResp := utils.JsonResponse{}

	data := jsonResp.Failure(utils.AppNotInstall, "应用未安装")
	_, _ = ctx.Write([]byte(data))
}

// IP验证, 通过反向代理访问gocron，需设置Header X-Real-IP才能获取到客户端真实IP
func ipAuth(ctx *macaron.Context) {
	if !app.Installed {
		return
	}
	allowIpsStr := app.Setting.AllowIps
	if allowIpsStr == "" {
		return
	}
	clientIp := ctx.RemoteAddr()
	allowIps := strings.Split(allowIpsStr, ",")
	if utils.InStringSlice(allowIps, clientIp) {
		return
	}
	logger.Warnf("非法IP访问-%s", clientIp)
	jsonResp := utils.JsonResponse{}

	data := jsonResp.Failure(utils.UnauthorizedError, "您无权限访问")

	_, _ = ctx.Write([]byte(data))
}

// 用户认证
func userAuth(ctx *macaron.Context) {
	if !app.Installed {
		return
	}
	userHandler := new(handler.User)
	_ = userHandler.RestoreToken(ctx)
	if userHandler.IsLogin(ctx) {
		return
	}
	uri := strings.TrimRight(ctx.Req.URL.Path, "/")
	if strings.HasPrefix(uri, "/v1") {
		return
	}
	excludePaths := []string{"", "/user/login", "/install/status"}
	for _, path := range excludePaths {
		if uri == path {
			return
		}
	}
	jsonResp := utils.JsonResponse{}
	data := jsonResp.Failure(utils.AuthError, "认证失败")
	_, _ = ctx.Write([]byte(data))

}

// URL权限验证
func urlAuth(ctx *macaron.Context) {

	userHandler := new(handler.User)

	if !app.Installed {
		return
	}
	if userHandler.IsAdmin(ctx) {
		return
	}
	uri := strings.TrimRight(ctx.Req.URL.Path, "/")
	if strings.HasPrefix(uri, "/v1") {
		return
	}
	// 普通用户允许访问的URL地址
	allowPaths := []string{
		"",
		"/install/status",
		"/task",
		"/task/log",
		"/host",
		"/host/all",
		"/user/login",
		"/user/editMyPassword",
	}
	for _, path := range allowPaths {
		if path == uri {
			return
		}
	}

	jsonResp := utils.JsonResponse{}

	data := jsonResp.Failure(utils.UnauthorizedError, "您无权限访问")
	_, _ = ctx.Write([]byte(data))
}

/** API接口签名验证 **/
func apiAuth(ctx *macaron.Context) {
	if !app.Installed {
		return
	}
	if !app.Setting.ApiSignEnable {
		return
	}
	apiKey := strings.TrimSpace(app.Setting.ApiKey)
	apiSecret := strings.TrimSpace(app.Setting.ApiSecret)
	json := utils.JsonResponse{}
	if apiKey == "" || apiSecret == "" {
		msg := json.CommonFailure("使用API前, 请先配置密钥")
		_, _ = ctx.Write([]byte(msg))
		return
	}
	currentTimestamp := time.Now().Unix()
	queryInt64 := ctx.QueryInt64("time")
	if queryInt64 <= 0 {
		msg := json.CommonFailure("参数time不能为空")
		_, _ = ctx.Write([]byte(msg))
		return
	}
	if queryInt64 < (currentTimestamp - 1800) {
		msg := json.CommonFailure("time无效")
		_, _ = ctx.Write([]byte(msg))
		return
	}
	sign := ctx.QueryTrim("sign")
	if sign == "" {
		msg := json.CommonFailure("参数sign不能为空")
		_, _ = ctx.Write([]byte(msg))
		return
	}
	raw := apiKey + strconv.FormatInt(queryInt64, 10) + strings.TrimSpace(ctx.Req.URL.Path) + apiSecret
	realSign := utils.Md5(raw)
	if sign != realSign {
		msg := json.CommonFailure("签名验证失败")
		_, _ = ctx.Write([]byte(msg))
		return
	}
}

// endregion
