package service

import (
	"context"

	"github.com/google/wire"
	"github.com/xh-polaris/deyu-core-api/biz/adaptor"
	"github.com/xh-polaris/deyu-core-api/biz/application/dto/core_api"
	dm "github.com/xh-polaris/deyu-core-api/biz/domain/model"
	"github.com/xh-polaris/deyu-core-api/biz/infra/cst"
	"github.com/xh-polaris/deyu-core-api/biz/infra/mapper/conversation"
	mmsg "github.com/xh-polaris/deyu-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util/logx"
)

type IConversationService interface {
	CreateConversation(ctx context.Context, req *core_api.CreateConversationReq) (*core_api.CreateConversationResp, error)
	RenameConversation(ctx context.Context, req *core_api.RenameConversationReq) (*core_api.RenameConversationResp, error)
	ListConversation(ctx context.Context, req *core_api.ListConversationReq) (*core_api.ListConversationResp, error)
	GetConversation(ctx context.Context, req *core_api.GetConversationReq) (*core_api.GetConversationResp, error)
	DeleteConversation(ctx context.Context, req *core_api.DeleteConversationReq) (*core_api.DeleteConversationResp, error)
	SearchConversation(ctx context.Context, req *core_api.SearchConversationReq) (*core_api.SearchConversationResp, error)
}

type ConversationService struct {
	ConversationMapper conversation.MongoMapper
	MessageMapper      mmsg.MongoMapper
}

var ConversationServiceSet = wire.NewSet(
	wire.Struct(new(ConversationService), "*"),
	wire.Bind(new(IConversationService), new(*ConversationService)),
)

func (s *ConversationService) CreateConversation(ctx context.Context, req *core_api.CreateConversationReq) (*core_api.CreateConversationResp, error) {
	// 鉴权
	uid, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logx.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}

	// 调用mapper创建对话
	newConversation, err := s.ConversationMapper.CreateNewConversation(ctx, uid)
	if err != nil {
		logx.Error("create conversation error: %v", err)
		return nil, cst.ConversationCreationErr
	}

	// 返回conversationID
	return &core_api.CreateConversationResp{Resp: util.Success(), ConversationId: newConversation.ConversationId.Hex()}, nil
}

func (s *ConversationService) RenameConversation(ctx context.Context, req *core_api.RenameConversationReq) (*core_api.RenameConversationResp, error) {
	// 鉴权
	uid, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logx.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}

	// 更新对话描述
	if err = s.ConversationMapper.UpdateConversationBrief(ctx, uid, req.GetConversationId(), req.GetBrief()); err != nil {
		logx.Error("update conversation brief error: %v", err)
		return nil, cst.ConversationRenameErr
	}

	// 返回响应
	return &core_api.RenameConversationResp{Resp: util.Success()}, nil
}

func (s *ConversationService) ListConversation(ctx context.Context, req *core_api.ListConversationReq) (*core_api.ListConversationResp, error) {
	// 鉴权
	uid, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logx.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}

	// 分页获取Conversation列表，并转化为ListConversationResp_ConversationItem
	conversations, hasMore, err := s.ConversationMapper.ListConversations(ctx, uid, req.GetPage())
	if err != nil {
		logx.Error("list conversation error: %v", err)
		return nil, cst.ConversationListErr
	}
	items := make([]*core_api.Conversation, len(conversations))
	for i, conv := range conversations {
		items[i] = &core_api.Conversation{
			ConversationId: conv.ConversationId.Hex(),
			Brief:          conv.Brief,
			CreateTime:     conv.CreateTime.Unix(),
			UpdateTime:     conv.UpdateTime.Unix(),
		}
	}

	resp := &core_api.ListConversationResp{Resp: util.Success(), Conversations: items, HasMore: hasMore}
	if len(conversations) > 0 {
		resp.Cursor = conversations[len(conversations)-1].ConversationId.Hex()
	}
	// 返回响应
	return resp, nil
}

func (s *ConversationService) GetConversation(ctx context.Context, req *core_api.GetConversationReq) (*core_api.GetConversationResp, error) {
	// 鉴权 optimize
	_, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logx.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}

	msgs, hasMore, err := s.MessageMapper.ListMessage(ctx, req.GetConversationId(), req.GetPage())
	if err != nil {
		logx.Error("get conversation messages error: %v", err)
		return nil, cst.ConversationGetErr
	}
	// 判断是否有regen
	var regen []*mmsg.Message
	if len(msgs) > 0 {
		replyId := msgs[0].ReplyId.Hex()
		for _, msg := range msgs[1:] {
			if msg.ReplyId.Hex() == replyId {
				if regen == nil {
					regen = []*mmsg.Message{msgs[0]}
				}
				regen = append(regen, msg)
			}
		}
	}
	resp := &core_api.GetConversationResp{
		Resp:        util.Success(),
		MessageList: dm.MMsgToFMsgList(msgs),
		RegenList:   dm.MMsgToFMsgList(regen),
		HasMore:     hasMore,
	}
	if len(resp.MessageList) > 0 {
		resp.Cursor = msgs[len(msgs)-1].MessageId.Hex()
	}
	return resp, nil
}

func (s *ConversationService) DeleteConversation(ctx context.Context, req *core_api.DeleteConversationReq) (*core_api.DeleteConversationResp, error) {
	uid, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logx.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}
	if err = s.ConversationMapper.DeleteConversation(ctx, uid, req.ConversationId); err != nil {
		logx.Error("delete conversation error: %v", err)
		return nil, cst.ConversationDeleteErr
	}
	return &core_api.DeleteConversationResp{Resp: util.Success()}, nil
}

func (s *ConversationService) SearchConversation(ctx context.Context, req *core_api.SearchConversationReq) (*core_api.SearchConversationResp, error) {
	// 鉴权
	uid, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logx.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}

	// 分页获取存储域Conversation列表，并转化为交互域中Conversation
	conversations, hasMore, err := s.ConversationMapper.SearchConversations(ctx, uid, req.GetKey(), req.GetPage())
	if err != nil {
		logx.Error("list conversation error: %v", err)
		return nil, cst.ConversationSearchErr
	}
	items := make([]*core_api.Conversation, len(conversations))
	for i, conv := range conversations {
		items[i] = &core_api.Conversation{
			ConversationId: conv.ConversationId.Hex(),
			Brief:          conv.Brief,
			CreateTime:     conv.CreateTime.Unix(),
			UpdateTime:     conv.UpdateTime.Unix(),
		}
	}

	resp := &core_api.SearchConversationResp{Resp: util.Success(), Conversations: items, HasMore: hasMore}
	if len(conversations) > 0 {
		resp.Cursor = conversations[len(conversations)-1].ConversationId.Hex()
	}
	// 返回响应
	return resp, nil
}
