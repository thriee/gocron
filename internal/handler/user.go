package handler

import (
	"errors"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/thriee/gocron/internal/dto"
	"github.com/thriee/gocron/internal/models"
	"github.com/thriee/gocron/internal/pkg/app"
	"github.com/thriee/gocron/internal/pkg/logger"
	"github.com/thriee/gocron/internal/pkg/utils"
	"gopkg.in/macaron.v1"
)

const tokenDuration = 4 * time.Hour

type User struct{}

// Index 用户列表页
func (u *User) Index(ctx *macaron.Context) string {
	queryParams := u.parseQueryParams(ctx)
	userModel := new(models.User)
	users, err := userModel.List(queryParams)
	if err != nil {
		logger.Error(err)
	}
	total, err := userModel.Total()
	if err != nil {
		logger.Error(err)
	}

	jsonResp := utils.JsonResponse{}

	return jsonResp.Success(utils.SuccessContent, map[string]interface{}{
		"total": total,
		"data":  users,
	})
}

// 解析查询参数
func (u *User) parseQueryParams(ctx *macaron.Context) models.CommonMap {
	params := models.CommonMap{}
	ParsePageAndPageSize(ctx, params)

	return params
}

// Detail 用户详情
func (u *User) Detail(ctx *macaron.Context) string {
	userModel := new(models.User)
	id := ctx.ParamsInt(":id")
	err := userModel.Find(id)
	if err != nil {
		logger.Error(err)
	}
	jsonResp := utils.JsonResponse{}
	if userModel.Id == 0 {
		return jsonResp.Success(utils.SuccessContent, nil)
	}

	return jsonResp.Success(utils.SuccessContent, userModel)
}

// Store 保存任务
func (u *User) Store(ctx *macaron.Context, form dto.UserForm) string {
	_ = ctx
	form.Name = strings.TrimSpace(form.Name)
	form.Email = strings.TrimSpace(form.Email)
	form.Password = strings.TrimSpace(form.Password)
	form.ConfirmPassword = strings.TrimSpace(form.ConfirmPassword)
	json := utils.JsonResponse{}
	userModel := models.User{}
	nameExists, err := userModel.UsernameExists(form.Name, form.Id)
	if err != nil {
		return json.CommonFailure(utils.FailureContent, err)
	}
	if nameExists > 0 {
		return json.CommonFailure("用户名已存在")
	}

	emailExists, err := userModel.EmailExists(form.Email, form.Id)
	if err != nil {
		return json.CommonFailure(utils.FailureContent, err)
	}
	if emailExists > 0 {
		return json.CommonFailure("邮箱已存在")
	}

	if form.Id == 0 {
		if form.Password == "" {
			return json.CommonFailure("请输入密码")
		}
		if form.ConfirmPassword == "" {
			return json.CommonFailure("请再次输入密码")
		}
		if form.Password != form.ConfirmPassword {
			return json.CommonFailure("两次密码输入不一致")
		}
	}
	userModel.Name = form.Name
	userModel.Email = form.Email
	userModel.Password = form.Password
	userModel.IsAdmin = form.IsAdmin
	userModel.Status = form.Status

	if form.Id == 0 {
		_, err = userModel.Create()
		if err != nil {
			return json.CommonFailure("添加失败", err)
		}
	} else {
		_, err = userModel.Update(form.Id, models.CommonMap{
			"name":     form.Name,
			"email":    form.Email,
			"status":   form.Status,
			"is_admin": form.IsAdmin,
		})
		if err != nil {
			return json.CommonFailure("修改失败", err)
		}
	}

	return json.Success("保存成功", nil)
}

// Remove 删除用户
func (u *User) Remove(ctx *macaron.Context) string {
	id := ctx.ParamsInt(":id")
	json := utils.JsonResponse{}

	userModel := new(models.User)
	_, err := userModel.Delete(id)
	if err != nil {
		return json.CommonFailure(utils.FailureContent, err)
	}

	return json.Success(utils.SuccessContent, nil)
}

// Enable 激活用户
func (u *User) Enable(ctx *macaron.Context) string {
	return u.changeStatus(ctx, models.Enabled)
}

// Disable 禁用用户
func (u *User) Disable(ctx *macaron.Context) string {
	return u.changeStatus(ctx, models.Disabled)
}

// 改变任务状态
func (u *User) changeStatus(ctx *macaron.Context, status models.Status) string {
	id := ctx.ParamsInt(":id")
	json := utils.JsonResponse{}
	userModel := new(models.User)
	_, err := userModel.Update(id, models.CommonMap{
		"status": status,
	})
	if err != nil {
		return json.CommonFailure(utils.FailureContent, err)
	}

	return json.Success(utils.SuccessContent, nil)
}

// UpdatePassword 更新密码
func (u *User) UpdatePassword(ctx *macaron.Context) string {
	id := ctx.ParamsInt(":id")
	newPassword := ctx.QueryTrim("new_password")
	confirmNewPassword := ctx.QueryTrim("confirm_new_password")
	json := utils.JsonResponse{}
	if newPassword == "" || confirmNewPassword == "" {
		return json.CommonFailure("请输入密码")
	}
	if newPassword != confirmNewPassword {
		return json.CommonFailure("两次输入密码不一致")
	}
	userModel := new(models.User)
	_, err := userModel.UpdatePassword(id, newPassword)
	if err != nil {
		return json.CommonFailure("修改失败")
	}

	return json.Success("修改成功", nil)
}

// UpdateMyPassword 更新我的密码
func (u *User) UpdateMyPassword(ctx *macaron.Context) string {
	oldPassword := ctx.QueryTrim("old_password")
	newPassword := ctx.QueryTrim("new_password")
	confirmNewPassword := ctx.QueryTrim("confirm_new_password")
	json := utils.JsonResponse{}
	if oldPassword == "" || newPassword == "" || confirmNewPassword == "" {
		return json.CommonFailure("原密码和新密码均不能为空")
	}
	if newPassword != confirmNewPassword {
		return json.CommonFailure("两次输入密码不一致")
	}
	if oldPassword == newPassword {
		return json.CommonFailure("原密码与新密码不能相同")
	}
	userModel := new(models.User)
	if !userModel.Match(u.Username(ctx), oldPassword) {
		return json.CommonFailure("原密码输入错误")
	}
	_, err := userModel.UpdatePassword(u.Uid(ctx), newPassword)
	if err != nil {
		return json.CommonFailure("修改失败")
	}

	return json.Success("修改成功", nil)
}

// ValidateLogin 验证用户登录
func (u *User) ValidateLogin(ctx *macaron.Context) string {
	username := ctx.QueryTrim("username")
	password := ctx.QueryTrim("password")
	json := utils.JsonResponse{}
	if username == "" || password == "" {
		return json.CommonFailure("用户名、密码不能为空")
	}
	userModel := new(models.User)
	if !userModel.Match(username, password) {
		return json.CommonFailure("用户名或密码错误")
	}
	loginLogModel := new(models.LoginLog)
	loginLogModel.Username = userModel.Name
	loginLogModel.Ip = ctx.RemoteAddr()
	_, err := loginLogModel.Create()
	if err != nil {
		logger.Error("记录用户登录日志失败", err)
	}

	token, err := u.generateToken(userModel)
	if err != nil {
		logger.Errorf("生成jwt失败: %s", err)
		return json.Failure(utils.AuthError, "认证失败")
	}

	return json.Success(utils.SuccessContent, map[string]interface{}{
		"token":    token,
		"uid":      userModel.Id,
		"username": userModel.Name,
		"is_admin": userModel.IsAdmin,
	})
}

// Username 获取session中的用户名
func (u *User) Username(ctx *macaron.Context) string {
	usernameInterface, ok := ctx.Data["username"]
	if !ok {
		return ""
	}
	if username, ok := usernameInterface.(string); ok {
		return username
	} else {
		return ""
	}
}

// Uid 获取session中的Uid
func (u *User) Uid(ctx *macaron.Context) int {
	uidInterface, ok := ctx.Data["uid"]
	if !ok {
		return 0
	}
	if uid, ok := uidInterface.(int); ok {
		return uid
	} else {
		return 0
	}
}

// IsLogin 判断用户是否已登录
func (u *User) IsLogin(ctx *macaron.Context) bool {
	return u.Uid(ctx) > 0
}

// IsAdmin 判断当前用户是否是管理员
func (u *User) IsAdmin(ctx *macaron.Context) bool {
	isAdmin, ok := ctx.Data["is_admin"]
	if !ok {
		return false
	}
	if v, ok := isAdmin.(int); ok {
		return v > 0
	} else {
		return false
	}
}

// 生成jwt
func (u *User) generateToken(user *models.User) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(tokenDuration).Unix()
	claims["uid"] = user.Id
	claims["iat"] = time.Now().Unix()
	claims["issuer"] = "gocron"
	claims["username"] = user.Name
	claims["is_admin"] = user.IsAdmin
	token.Claims = claims

	return token.SignedString([]byte(app.Setting.AuthSecret))
}

// RestoreToken 还原jwt
func (u *User) RestoreToken(ctx *macaron.Context) error {
	authToken := ctx.Req.Header.Get("Auth-Token")
	if authToken == "" {
		return nil
	}
	token, err := jwt.Parse(authToken, func(*jwt.Token) (interface{}, error) {
		return []byte(app.Setting.AuthSecret), nil
	})
	if err != nil {
		return err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("invalid claims")
	}
	ctx.Data["uid"] = int(claims["uid"].(float64))
	ctx.Data["username"] = claims["username"]
	ctx.Data["is_admin"] = int(claims["is_admin"].(float64))

	return nil
}
