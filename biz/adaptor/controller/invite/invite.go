package invite

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/deyu-core-api/biz/adaptor"
	"github.com/xh-polaris/deyu-core-api/provider"
)

type CheckReq struct {
	Code  string `json:"code" vd:"len($)>0"`
	Phone string `json:"phone" vd:"len($)>0"`
}

type GenReq struct {
	MaxCount int `json:"max_count" vd:"$>0"`
}

func Check(ctx context.Context, c *app.RequestContext) {
	var req CheckReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	resp, err := provider.Get().InviteService.Check(ctx, req.Code, req.Phone)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}

func Gen(ctx context.Context, c *app.RequestContext) {
	var req GenReq
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	resp, err := provider.Get().InviteService.Gen(ctx, req.MaxCount)
	adaptor.PostProcess(ctx, c, &req, resp, err)
}
