package model

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/xh-polaris/deyu-core-api/biz/application/dto/core_api"
)

type getModelFunc func(ctx context.Context, uid string, req *core_api.CompletionsReq) (model.ToolCallingChatModel, error)

var models = map[string]getModelFunc{}

func RegisterModel(name string, f getModelFunc) {
	models[name] = f
}

// GetModel 获取模型
func GetModel(ctx context.Context, uid string, req *core_api.CompletionsReq) (model.ToolCallingChatModel, error) {
	return models[req.Model](ctx, uid, req)
}
