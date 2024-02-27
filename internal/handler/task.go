package handler

import (
	"strconv"
	"strings"

	"github.com/ouqiang/goutil"
	"github.com/thriee/gocron/internal/dto"

	"github.com/jakecoffman/cron"
	"github.com/thriee/gocron/internal/models"
	"github.com/thriee/gocron/internal/pkg/logger"
	"github.com/thriee/gocron/internal/pkg/utils"
	"github.com/thriee/gocron/internal/service"
	"gopkg.in/macaron.v1"
)

type Task struct{}

// Index 首页
func (t *Task) Index(ctx *macaron.Context) string {
	taskModel := new(models.Task)
	queryParams := t.parseQueryParams(ctx)
	total, err := taskModel.Total(queryParams)
	if err != nil {
		logger.Error(err)
	}
	tasks, err := taskModel.List(queryParams)
	if err != nil {
		logger.Error(err)
	}
	for i, item := range tasks {
		tasks[i].NextRunTime = service.ServiceTask.NextRunTime(item)
	}
	jsonResp := utils.JsonResponse{}

	return jsonResp.Success(utils.SuccessContent, map[string]interface{}{
		"total": total,
		"data":  tasks,
	})
}

// Detail 任务详情
func (t *Task) Detail(ctx *macaron.Context) string {
	id := ctx.ParamsInt(":id")
	taskModel := new(models.Task)
	task, err := taskModel.Detail(id)
	jsonResp := utils.JsonResponse{}
	if err != nil || task.Id == 0 {
		logger.Errorf("编辑任务#获取任务详情失败#任务ID-%d", id)
		return jsonResp.Success(utils.SuccessContent, nil)
	}

	return jsonResp.Success(utils.SuccessContent, task)
}

// Store 保存任务  todo 拆分为多个方法
func (t *Task) Store(ctx *macaron.Context, form dto.TaskForm) string {
	_ = ctx
	json := utils.JsonResponse{}
	taskModel := models.Task{}
	var id = form.Id
	nameExists, err := taskModel.NameExist(form.Name, form.Id)
	if err != nil {
		return json.CommonFailure(utils.FailureContent, err)
	}
	if nameExists {
		return json.CommonFailure("任务名称已存在")
	}

	if form.Protocol == models.TaskRPC && form.HostId == "" {
		return json.CommonFailure("请选择主机名")
	}

	taskModel.Name = form.Name
	taskModel.Protocol = form.Protocol
	taskModel.Command = strings.TrimSpace(form.Command)
	taskModel.Timeout = form.Timeout
	taskModel.Tag = form.Tag
	taskModel.Remark = form.Remark
	taskModel.Multi = form.Multi
	taskModel.RetryTimes = form.RetryTimes
	taskModel.RetryInterval = form.RetryInterval
	if taskModel.Multi != 1 {
		taskModel.Multi = 0
	}
	taskModel.NotifyStatus = form.NotifyStatus - 1
	taskModel.NotifyType = form.NotifyType - 1
	taskModel.NotifyReceiverId = form.NotifyReceiverId
	taskModel.NotifyKeyword = form.NotifyKeyword
	taskModel.Spec = form.Spec
	taskModel.Level = form.Level
	taskModel.DependencyStatus = form.DependencyStatus
	taskModel.DependencyTaskId = strings.TrimSpace(form.DependencyTaskId)
	if taskModel.NotifyStatus > 0 && taskModel.NotifyType != 3 && taskModel.NotifyReceiverId == "" {
		return json.CommonFailure("至少选择一个通知接收者")
	}
	taskModel.HttpMethod = form.HttpMethod
	if taskModel.Protocol == models.TaskHTTP {
		command := strings.ToLower(taskModel.Command)
		if !strings.HasPrefix(command, "http://") && !strings.HasPrefix(command, "https://") {
			return json.CommonFailure("请输入正确的URL地址")
		}
		if taskModel.Timeout > 300 {
			return json.CommonFailure("HTTP任务超时时间不能超过300秒")
		}
	}

	if taskModel.RetryTimes > 10 || taskModel.RetryTimes < 0 {
		return json.CommonFailure("任务重试次数取值0-10")
	}

	if taskModel.RetryInterval > 3600 || taskModel.RetryInterval < 0 {
		return json.CommonFailure("任务重试间隔时间取值0-3600")
	}

	if taskModel.DependencyStatus != models.TaskDependencyStatusStrong &&
		taskModel.DependencyStatus != models.TaskDependencyStatusWeak {
		return json.CommonFailure("请选择依赖关系")
	}

	if taskModel.Level == models.TaskLevelParent {
		err = goutil.PanicToError(func() {
			cron.Parse(form.Spec)
		})
		if err != nil {
			return json.CommonFailure("crontab表达式解析失败", err)
		}
	} else {
		taskModel.DependencyTaskId = ""
		taskModel.Spec = ""
	}

	if id > 0 && taskModel.DependencyTaskId != "" {
		dependencyTaskIds := strings.Split(taskModel.DependencyTaskId, ",")
		if utils.InStringSlice(dependencyTaskIds, strconv.Itoa(id)) {
			return json.CommonFailure("不允许设置当前任务为子任务")
		}
	}

	if id == 0 {
		// 任务添加后开始调度执行
		taskModel.Status = models.Running
		id, err = taskModel.Create()
	} else {
		_, err = taskModel.UpdateBean(id)
	}

	if err != nil {
		return json.CommonFailure("保存失败", err)
	}

	taskHostModel := new(models.TaskHost)
	if form.Protocol == models.TaskRPC {
		hostIdStrList := strings.Split(form.HostId, ",")
		hostIds := make([]int, len(hostIdStrList))
		for i, hostIdStr := range hostIdStrList {
			hostIds[i], _ = strconv.Atoi(hostIdStr)
		}
		taskHostModel.Add(id, hostIds)
	} else {
		taskHostModel.Remove(id)
	}

	status, _ := taskModel.GetStatus(id)
	if status == models.Enabled && taskModel.Level == models.TaskLevelParent {
		t.addTaskToTimer(id)
	}

	return json.Success("保存成功", nil)
}

// Remove 删除任务
func (t *Task) Remove(ctx *macaron.Context) string {
	id := ctx.ParamsInt(":id")
	json := utils.JsonResponse{}
	taskModel := new(models.Task)
	_, err := taskModel.Delete(id)
	if err != nil {
		return json.CommonFailure(utils.FailureContent, err)
	}

	taskHostModel := new(models.TaskHost)
	_ = taskHostModel.Remove(id)

	service.ServiceTask.Remove(id)

	return json.Success(utils.SuccessContent, nil)
}

// Enable 激活任务
func (t *Task) Enable(ctx *macaron.Context) string {
	return t.changeStatus(ctx, models.Enabled)
}

// Disable 暂停任务
func (t *Task) Disable(ctx *macaron.Context) string {
	return t.changeStatus(ctx, models.Disabled)
}

// Run 手动运行任务
func (t *Task) Run(ctx *macaron.Context) string {
	id := ctx.ParamsInt(":id")
	json := utils.JsonResponse{}
	taskModel := new(models.Task)
	task, err := taskModel.Detail(id)
	if err != nil || task.Id <= 0 {
		return json.CommonFailure("获取任务详情失败", err)
	}

	task.Spec = "手动运行"
	service.ServiceTask.Run(task)

	return json.Success("任务已开始运行, 请到任务日志中查看结果", nil)
}

// 改变任务状态
func (t *Task) changeStatus(ctx *macaron.Context, status models.Status) string {
	id := ctx.ParamsInt(":id")
	json := utils.JsonResponse{}
	taskModel := new(models.Task)
	_, err := taskModel.Update(id, models.CommonMap{
		"status": status,
	})
	if err != nil {
		return json.CommonFailure(utils.FailureContent, err)
	}

	if status == models.Enabled {
		t.addTaskToTimer(id)
	} else {
		service.ServiceTask.Remove(id)
	}

	return json.Success(utils.SuccessContent, nil)
}

// 添加任务到定时器
func (t *Task) addTaskToTimer(id int) {
	taskModel := new(models.Task)
	task, err := taskModel.Detail(id)
	if err != nil {
		logger.Error(err)
		return
	}

	service.ServiceTask.RemoveAndAdd(task)
}

// 解析查询参数
func (t *Task) parseQueryParams(ctx *macaron.Context) models.CommonMap {
	var params = models.CommonMap{}
	params["Id"] = ctx.QueryInt("id")
	params["HostId"] = ctx.QueryInt("host_id")
	params["Name"] = ctx.QueryTrim("name")
	params["Protocol"] = ctx.QueryInt("protocol")
	params["Tag"] = ctx.QueryTrim("tag")
	status := ctx.QueryInt("status")
	if status >= 0 {
		status -= 1
	}
	params["Status"] = status
	ParsePageAndPageSize(ctx, params)

	return params
}
