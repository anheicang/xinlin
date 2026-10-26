package wxapi

import "github.com/anheicang/xinlin/base/baseerror"

var (
	ErrGetUserInfoFail = baseerror.NewCodeError(1001, "获取用户基本信息失败")
	ErrGetTokenFail    = baseerror.NewCodeError(1002, "获取应用令牌失败")
)
