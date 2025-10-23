package cmd

import "github.com/xh-polaris/deyu-core-api/biz/application/dto/basic"

type SetPasswordReq struct {
	NewPassword string `json:"newPassword"`
}

type SetPasswordResp struct {
	Resp *basic.Response `json:"resp"`
}
