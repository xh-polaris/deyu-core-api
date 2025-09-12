package provider

import (
	"github.com/google/wire"
	"github.com/xh-polaris/deyu-core-api/biz/application/service"
	"github.com/xh-polaris/deyu-core-api/biz/domain/model"
	"github.com/xh-polaris/deyu-core-api/biz/infra/config"
	"github.com/xh-polaris/deyu-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/deyu-core-api/biz/infra/mapper/feedback"
	"github.com/xh-polaris/deyu-core-api/biz/infra/mapper/message"
)

var provider *Provider

func Init() {
	var err error
	provider, err = NewProvider()
	if err != nil {
		panic(err)
	}
}

// Provider 提供controller依赖的对象
type Provider struct {
	Config              *config.Config
	CompletionsService  service.ICompletionsService
	ConversationService service.IConversationService
	FeedbackService     service.IFeedbackService
	MessageDomain       *model.MessageDomain
	CompletionDomain    *model.CompletionDomain
}

func Get() *Provider {
	return provider
}

var RPCSet = wire.NewSet()

var ApplicationSet = wire.NewSet(
	service.CompletionsServiceSet,
	service.ConversationServiceSet,
	service.FeedbackServiceSet,
)

var DomainSet = wire.NewSet(
	model.MessageDomainSet,
	model.CompletionDomainSet,
)

var InfraSet = wire.NewSet(
	config.NewConfig,
	RPCSet,
	conversation.NewConversationMongoMapper,
	message.NewMessageMongoMapper,
	feedback.NewFeedbackMongoMapper,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	DomainSet,
	InfraSet,
)
