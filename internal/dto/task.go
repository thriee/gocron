package dto

import (
	"github.com/go-macaron/binding"
	"github.com/thriee/gocron/internal/models"
	"github.com/thriee/gocron/internal/pkg/utils"
	"gopkg.in/macaron.v1"
)

type TaskForm struct {
	Id               int
	Level            models.TaskLevel `binding:"Required;In(1,2)"`
	DependencyStatus models.TaskDependencyStatus
	DependencyTaskId string
	Name             string `binding:"Required;MaxSize(32)"`
	Spec             string
	Protocol         models.TaskProtocol   `binding:"In(1,2)"`
	Command          string                `binding:"Required;MaxSize(256)"`
	HttpMethod       models.TaskHTTPMethod `binding:"In(1,2)"`
	Timeout          int                   `binding:"Range(0,86400)"`
	Multi            int8                  `binding:"In(1,2)"`
	RetryTimes       int8
	RetryInterval    int16
	HostId           string
	Tag              string
	Remark           string
	NotifyStatus     int8 `binding:"In(1,2,3,4)"`
	NotifyType       int8 `binding:"In(1,2,3,4)"`
	NotifyReceiverId string
	NotifyKeyword    string
}

func (f TaskForm) Error(ctx *macaron.Context, errs binding.Errors) {
	if len(errs) == 0 {
		return
	}
	json := utils.JsonResponse{}
	content := json.CommonFailure("表单验证失败, 请检测输入")

	ctx.Resp.Write([]byte(content))
}
