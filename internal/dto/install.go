package dto

import (
	"github.com/go-macaron/binding"
	"github.com/thriee/gocron/internal/pkg/utils"
	"gopkg.in/macaron.v1"
)

type InstallForm struct {
	DbType               string `binding:"In(mysql,postgres)"`
	DbHost               string `binding:"Required;MaxSize(50)"`
	DbPort               int    `binding:"Required;Range(1,65535)"`
	DbUsername           string `binding:"Required;MaxSize(50)"`
	DbPassword           string `binding:"Required;MaxSize(30)"`
	DbName               string `binding:"Required;MaxSize(50)"`
	DbTablePrefix        string `binding:"MaxSize(20)"`
	AdminUsername        string `binding:"Required;MinSize(3)"`
	AdminPassword        string `binding:"Required;MinSize(6)"`
	ConfirmAdminPassword string `binding:"Required;MinSize(6)"`
	AdminEmail           string `binding:"Required;Email;MaxSize(50)"`
}

func (f InstallForm) Error(ctx *macaron.Context, errs binding.Errors) {
	if len(errs) == 0 {
		return
	}
	json := utils.JsonResponse{}
	content := json.CommonFailure("表单验证失败, 请检测输入")
	_, _ = ctx.Write([]byte(content))
}
