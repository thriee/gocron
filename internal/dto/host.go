package dto

import (
	"github.com/go-macaron/binding"
	"github.com/thriee/gocron/internal/pkg/utils"
	"gopkg.in/macaron.v1"
)

type HostForm struct {
	Id     int16
	Name   string `binding:"Required;MaxSize(64)"`
	Alias  string `binding:"Required;MaxSize(32)"`
	Port   int    `binding:"Required;Range(1-65535)"`
	Remark string
}

// Error 表单验证错误处理
func (f HostForm) Error(ctx *macaron.Context, errs binding.Errors) {
	if len(errs) == 0 {
		return
	}
	json := utils.JsonResponse{}
	content := json.CommonFailure("表单验证失败, 请检测输入")
	_, _ = ctx.Write([]byte(content))
}
