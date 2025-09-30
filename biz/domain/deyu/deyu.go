package deyu

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/coze-go"
	"github.com/xh-polaris/deyu-core-api/biz/application/dto/core_api"
	dm "github.com/xh-polaris/deyu-core-api/biz/domain/model"
	"github.com/xh-polaris/deyu-core-api/biz/infra/config"
	"github.com/xh-polaris/deyu-core-api/biz/infra/cst"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util/logx"
)

func init() {
	dm.RegisterModel(DefaultModel, NewChatModel)
	dm.RegisterModel(BZRModel, NewChatModel)
	dm.RegisterModel(XKJSModel, NewChatModel)
	dm.RegisterModel(QYDSModel, NewChatModel)
	dm.RegisterModel(DYGB, NewChatModel)
	dm.RegisterModel(XYModel, NewChatModel)
	dm.RegisterModel(JYLFModel, NewChatModel)
}

var (
	cli  *openai.ChatModel
	once sync.Once

	DefaultModel = "deyu-default"
	BZRModel     = "deyu-bzr"
	XKJSModel    = "deyu-xkjs"
	QYDSModel    = "deyu-qyds"
	DYGB         = "deyu-dygb"
	XYModel      = "deyu-xy"
	JYLFModel    = "deyu-jylf"
	APIVersion   = "v1"
)

// ChatModel 德育大模型
// 在openai模型基础上封装
type ChatModel struct {
	cli     *openai.ChatModel
	cozeCli *coze.CozeAPI
	Model   string
	Uid     string
}

func NewChatModel(ctx context.Context, uid string, req *core_api.CompletionsReq) (_ model.ToolCallingChatModel, err error) {
	m := &ChatModel{Model: req.Model, Uid: uid}
	if req.Model != DefaultModel {
		cozeCli := coze.NewCozeAPI(coze.NewTokenAuth(config.GetConfig().Models[req.Model].APIKey),
			coze.WithBaseURL(config.GetConfig().Models[req.Model].BaseURL),
			coze.WithHttpClient(util.NewDebugClient()))
		m.cozeCli = &cozeCli
	} else {
		cli, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:      config.GetConfig().Models[req.Model].APIKey,
			BaseURL:     config.GetConfig().Models[req.Model].BaseURL,
			APIVersion:  APIVersion,
			Model:       config.GetConfig().Models[req.Model].Name,
			User:        &uid,
			HTTPClient:  util.NewDebugClient(),
			ExtraFields: map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": false}},
		})
		m.cli = cli
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (c *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// messages翻转顺序, 调用模型时消息应该正序
	var reverse []*schema.Message
	for i := len(in) - 1; i >= 0; i-- {
		reverse = append(reverse, in[i])
	}
	return c.cli.Generate(ctx, in, opts...)
}

func (c *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (sr *schema.StreamReader[*schema.Message], err error) {
	sr, sw := schema.Pipe[*schema.Message](5)
	// messages翻转顺序, 调用模型时消息应该正序
	var reverse []*schema.Message
	for i := len(in) - 1; i >= 0; i-- {
		in[i].Name = ""
		reverse = append(reverse, in[i])
	}
	if c.Model != DefaultModel {
		request := &coze.CreateChatsReq{
			BotID:    config.GetConfig().Models[c.Model].BotID,
			UserID:   c.Uid,
			Messages: e2c(reverse),
		}
		var stream coze.Stream[coze.ChatEvent]
		if stream, err = c.cozeCli.Chat.Stream(ctx, request); err != nil {
			return nil, err
		}
		go cozeProcess(ctx, stream, sw)
	} else {
		var reader *schema.StreamReader[*schema.Message]
		if reader, err = c.cli.Stream(ctx, reverse, opts...); err != nil {
			logx.Error("call %s err:%v", c.Model, err)
			return nil, err
		}
		go process(ctx, reader, sw)
	}
	return sr, nil
}

func e2c(in []*schema.Message) (c []*coze.Message) {
	for _, i := range in {
		m := &coze.Message{
			Role:             coze.MessageRole(i.Role),
			Content:          i.Content,
			ReasoningContent: i.ReasoningContent,
			Type:             "question",
			ContentType:      "text",
		}
		c = append(c, m)
	}
	return
}

func ce2e(e *coze.ChatEvent) *schema.Message {
	return c2e(e.Message)
}
func c2e(c *coze.Message) *schema.Message {
	return &schema.Message{
		Role:    schema.Assistant,
		Content: c.Content,
	}
}
func cozeProcess(ctx context.Context, reader coze.Stream[coze.ChatEvent], writer *schema.StreamWriter[*schema.Message]) {
	defer func() { _ = reader.Close() }()
	defer writer.Close()

	var err error
	var data []byte
	var event *coze.ChatEvent
	var msg *schema.Message

	var pass bool // 跳过一个\n\n
	var status = cst.EventMessageContentTypeText
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if event, err = reader.Recv(); err != nil {
				writer.Send(nil, err)
				return
			}
			if event.Message == nil || event.Event != coze.ChatEventConversationMessageDelta {
				continue
			}
			msg = ce2e(event)
			if pass && msg.Content == "\n\n" {
				pass = false
				continue
			}

			refine := &dm.RefineContent{}
			// 处理消息
			switch msg.Content {
			case cst.ThinkStart: // 深度思考内容开始
				status, pass = cst.EventMessageContentTypeThink, true
				continue
			case cst.SuggestStart: // 建议内容开始
				status, pass = cst.EventMessageContentTypeSuggest, true
				continue
			case cst.ThinkEnd:
				fallthrough // 切回普通内容
			case cst.SuggestEnd:
				status, pass = cst.EventMessageContentTypeText, true
				continue
			}
			switch status {
			case cst.EventMessageContentTypeText:
				refine.Text = msg.Content
			case cst.EventMessageContentTypeThink:
				refine.Think = msg.Content
			case cst.EventMessageContentTypeSuggest:
				refine.Suggest = msg.Content
			}
			if data, err = json.Marshal(&refine); err != nil {
				continue
			}
			msg.Content, msg.Extra = string(data), map[string]any{cst.EventMessageContentType: status, cst.RawMessage: msg.Content}
			writer.Send(msg, nil)
		}
	}
}

func process(ctx context.Context, reader *schema.StreamReader[*schema.Message], writer *schema.StreamWriter[*schema.Message]) {
	defer reader.Close()
	defer writer.Close()

	var err error
	var data []byte
	var msg *schema.Message

	var pass bool // 跳过一个\n\n
	var status = cst.EventMessageContentTypeText
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if msg, err = reader.Recv(); err != nil {
				writer.Send(nil, err)
				return
			}
			if pass && msg.Content == "\n\n" {
				pass = false
				continue
			}

			refine := &dm.RefineContent{}
			// 处理消息
			switch msg.Content {
			case cst.ThinkStart: // 深度思考内容开始
				status, pass = cst.EventMessageContentTypeThink, true
				continue
			case cst.SuggestStart: // 建议内容开始
				status, pass = cst.EventMessageContentTypeSuggest, true
				continue
			case cst.ThinkEnd:
				fallthrough // 切回普通内容
			case cst.SuggestEnd:
				status, pass = cst.EventMessageContentTypeText, true
				continue
			}
			switch status {
			case cst.EventMessageContentTypeText:
				refine.Text = msg.Content
			case cst.EventMessageContentTypeThink:
				refine.Think = msg.Content
			case cst.EventMessageContentTypeSuggest:
				refine.Suggest = msg.Content
			}
			if data, err = json.Marshal(&refine); err != nil {
				continue
			}
			msg.Content, msg.Extra = string(data), map[string]any{cst.EventMessageContentType: status, cst.RawMessage: msg.Content}
			writer.Send(msg, nil)
		}
	}
}

func (c *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return c.cli.WithTools(tools)
}
