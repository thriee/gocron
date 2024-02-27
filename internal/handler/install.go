package handler

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/thriee/gocron/internal/dto"
	"gopkg.in/macaron.v1"
	"xorm.io/xorm"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/thriee/gocron/internal/models"
	"github.com/thriee/gocron/internal/pkg/app"
	"github.com/thriee/gocron/internal/pkg/setting"
	"github.com/thriee/gocron/internal/pkg/utils"
	"github.com/thriee/gocron/internal/service"
)

type Install struct{}

// Store 安装
func (i *Install) Store(ctx *macaron.Context, form dto.InstallForm) string {
	_ = ctx
	json := utils.JsonResponse{}
	if app.Installed {
		return json.CommonFailure("系统已安装!")
	}
	if form.AdminPassword != form.ConfirmAdminPassword {
		return json.CommonFailure("两次输入密码不匹配")
	}
	err := i.testDbConnection(form)
	if err != nil {
		return json.CommonFailure(err.Error())
	}
	// 写入数据库配置
	err = i.writeConfig(form)
	if err != nil {
		return json.CommonFailure("数据库配置写入文件失败", err)
	}

	appConfig, err := setting.Read(app.AppConfig)
	if err != nil {
		return json.CommonFailure("读取应用配置失败", err)
	}
	app.Setting = appConfig

	models.Db = models.CreateDb()
	// 创建数据库表
	migration := new(models.Migration)
	err = migration.Install(form.DbName)
	if err != nil {
		return json.CommonFailure(fmt.Sprintf("创建数据库表失败-%s", err.Error()), err)
	}

	// 创建管理员账号
	err = i.createAdminUser(form)
	if err != nil {
		return json.CommonFailure("创建管理员账号失败", err)
	}

	// 创建安装锁
	err = app.CreateInstallLock()
	if err != nil {
		return json.CommonFailure("创建文件安装锁失败", err)
	}

	// 更新版本号文件
	app.UpdateVersionFile()

	app.Installed = true
	// 初始化定时任务
	service.ServiceTask.Initialize()

	return json.Success("安装成功", nil)
}

// 配置写入文件
func (i *Install) writeConfig(form dto.InstallForm) error {
	dbConfig := []string{
		"db.engine", form.DbType,
		"db.host", form.DbHost,
		"db.port", strconv.Itoa(form.DbPort),
		"db.user", form.DbUsername,
		"db.password", form.DbPassword,
		"db.database", form.DbName,
		"db.prefix", form.DbTablePrefix,
		"db.charset", "utf8",
		"db.max.idle.conns", "5",
		"db.max.open.conns", "100",
		"allow_ips", "",
		"app.name", "定时任务管理系统", // 应用名称
		"api.key", "",
		"api.secret", "",
		"enable_tls", "false",
		"concurrency.queue", "500",
		"auth_secret", utils.RandAuthToken(),
		"ca_file", "",
		"cert_file", "",
		"key_file", "",
	}

	return setting.Write(dbConfig, app.AppConfig)
}

// 创建管理员账号
func (i *Install) createAdminUser(form dto.InstallForm) error {
	user := new(models.User)
	user.Name = form.AdminUsername
	user.Password = form.AdminPassword
	user.Email = form.AdminEmail
	user.IsAdmin = 1
	_, err := user.Create()

	return err
}

// 测试数据库连接
func (i *Install) testDbConnection(form dto.InstallForm) error {
	var s setting.Setting
	s.Db.Engine = form.DbType
	s.Db.Host = form.DbHost
	s.Db.Port = form.DbPort
	s.Db.User = form.DbUsername
	s.Db.Password = form.DbPassword
	s.Db.Database = form.DbName
	s.Db.Charset = "utf8"
	db, err := models.CreateTmpDb(&s)
	if err != nil {
		return err
	}
	defer func(db *xorm.Engine) {
		err := db.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(db)
	err = db.Ping()
	if s.Db.Engine == "postgres" && err != nil {
		var pgError *pq.Error
		ok := errors.As(err, &pgError)
		if ok && pgError.Code == "3D000" {
			err = errors.New("数据库不存在")
		}
		return err
	}

	if s.Db.Engine == "mysql" && err != nil {
		var mysqlError *mysql.MySQLError
		ok := errors.As(err, &mysqlError)
		if ok && mysqlError.Number == 1049 {
			err = errors.New("数据库不存在")
		}
		return err
	}

	return err

}
