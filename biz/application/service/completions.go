package service

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/google/wire"
	"github.com/xh-polaris/deyu-core-api/biz/adaptor"
	"github.com/xh-polaris/deyu-core-api/biz/application/dto/core_api"
	_ "github.com/xh-polaris/deyu-core-api/biz/domain/deyu"
	"github.com/xh-polaris/deyu-core-api/biz/domain/model"
	"github.com/xh-polaris/deyu-core-api/biz/infra/cst"
	"github.com/xh-polaris/deyu-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util/logx"
)

type ICompletionsService interface {
	Completions(ctx context.Context, req *core_api.CompletionsReq) (any, error)
	GenerateBrief(ctx context.Context, req *core_api.GenerateBriefReq) (*core_api.GenerateBriefResp, error)
}

type CompletionsService struct {
	MsgMaMsgDomain     *model.MessageDomain
	CompletionDomain   *model.CompletionDomain
	ConversationMapper conversation.MongoMapper
}

var CompletionsServiceSet = wire.NewSet(
	wire.Struct(new(CompletionsService), "*"),
	wire.Bind(new(ICompletionsService), new(*CompletionsService)),
)

func (s *CompletionsService) Completions(ctx context.Context, req *core_api.CompletionsReq) (any, error) {
	// 鉴权
	uid, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logx.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}

	// 暂时只支持一个新增对话
	if len(req.Messages) > 1 {
		return nil, cst.UnImplementErr
	}

	// 构建聊天记录和info
	ctx, messages, info, err := s.MsgMaMsgDomain.GetMessagesAndInjectContext(ctx, uid, req)
	if err != nil {
		return nil, err
	}

	// 进行对话, 在最后更新历史记录
	return s.CompletionDomain.Completion(ctx, uid, req, messages, info)
}

func (s *CompletionsService) GenerateBrief(ctx context.Context, req *core_api.GenerateBriefReq) (*core_api.GenerateBriefResp, error) {
	// 鉴权
	uid, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logx.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}
	// 生成标题
	m, err := model.GetModel(ctx, uid, &core_api.CompletionsReq{
		Messages:          req.Messages,
		CompletionsOption: &core_api.CompletionsOption{},
		Model:             "deyu-default",
		ConversationId:    req.ConversationId,
	})
	if err != nil {
		return nil, err
	}
	in := []*schema.Message{schema.SystemMessage("你是标题生成器, 你需要根据这个用户输入生成一个简要标题, 不超过十个字"),
		schema.UserMessage(req.Messages[0].Content)}
	out, err := m.Generate(ctx, in)
	if err != nil {
		return nil, err
	}
	out.Content = strings.Replace(out.Content, "<think>\n\n</think>\n\n", "", -1)
	// 更新标题
	if err = s.ConversationMapper.UpdateConversationBrief(ctx, uid, req.ConversationId, out.Content); err != nil {
		return nil, err
	}
	return &core_api.GenerateBriefResp{Resp: util.Success(), Brief: out.Content}, nil
}
