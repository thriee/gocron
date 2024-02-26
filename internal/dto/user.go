package dto

import "github.com/thriee/gocron/internal/models"

// UserForm 用户表单
type UserForm struct {
	Id              int
	Name            string `binding:"Required;MaxSize(32)"` // 用户名
	Password        string // 密码
	ConfirmPassword string // 确认密码
	Email           string `binding:"Required;MaxSize(50)"` // 邮箱
	IsAdmin         int8   // 是否是管理员 1:管理员 0:普通用户
	Status          models.Status
}
