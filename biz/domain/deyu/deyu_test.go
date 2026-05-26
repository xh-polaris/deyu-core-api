package deyu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/coze-go"
)

func TestOpenAIFormat(t *testing.T) {
	model := getModel("https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen3-0.6b")
	messages := getMessages()
	stream, err := model.Stream(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var sb strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		sb.WriteString(chunk.Content)
		t.Logf("%+v\n", chunk)
	}
	t.Log(sb.String())
}

func TestConcurrentCallWithMultiModel(t *testing.T) {
	wg := sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int, t *testing.T) {
			model := getModel("https://edusys1.sii.edu.cn/deyu/14b/bzr_only/v1", "deyu-default")
			stream, err := model.Stream(context.Background(), getMessages())
			if err != nil {
				t.Errorf("%d|%s\n", i, err)
				return
			}
			defer stream.Close()
			var sb strings.Builder
			for {
				chunk, err := stream.Recv()
				if err != nil {
					break
				}
				sb.WriteString(chunk.Content)
				//t.Logf("%+v\n", chunk)
			}
			t.Logf("%d|%s\n", i, sb.String())
			wg.Done()
		}(i, t)
	}
	wg.Wait()
}

func getModel(url, name string) *openai.ChatModel {
	model, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: url, // Azure API 基础 URL
		// 基础配置
		APIKey: "sk-cc248d3f7dca4e25934ba0973bca357d", // API 密钥
		// 模型参数
		Model: name, // 模型名称
		//HTTPClient: &http.Client{Transport: util.NewLoggingTransport()},
	})
	if err != nil {
		panic(err)
	}
	return model
}

func getMessages() []*schema.Message {
	messages := []*schema.Message{
		// 系统消息
		schema.UserMessage("请你为我设计一节\"诚实面对错误\"的班会，对象是小学五年级学生，教学目标是让学生理解诚实的重要性，学会完整讲述事件经过，提升自我反思与沟通能力，不输出除此之外的东西")}
	return messages
}

// 德育班主任 https://edusys1.sii.edu.cn/deyu/14b/bzr/v1/
// 学科教师 https://edusys1.sii.edu.cn/deyu/14b/xkjs/v1/
// 全员导师 https://edusys1.sii.edu.cn/deyu/14b/qyds/v1/
// 德育干部 https://edusys1.sii.edu.cn/deyu/14b/dygb/v1/
// 心芽 https://edusys1.sii.edu.cn/deyu/14b/xy/v1/
// 家育良方 https://edusys1.sii.edu.cn/deyu/14b/jylf/v1/
func TestModel(t *testing.T) {
	url := "https://edusys1.sii.edu.cn/deyu/14b/bzr/v1/"
	name := "德育班主任"
	model := getModel(url, name)
	messages := getMessages()
	stream, err := model.Stream(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var sb strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
			break
		}
		t.Logf("time: %s | %+v\n", time.Now().String(), chunk)
		sb.WriteString(chunk.Content)
	}
	t.Log(sb.String())
}

func TestCoze(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get an access_token through personal access token or oauth.
	token := "pat_nVzwVNdkuMIpLSEA8NhpgjPpysvTRmThkg47y8J42jOnZKMhdpJ50gHNOFknQrKn"
	botID := "7553106253867352116"
	userID := "test"

	authCli := coze.NewTokenAuth(token)

	customClient := &http.Client{
		Timeout: time.Minute * 20,
	}

	// Init the Coze client through the access_token.
	cozeCli := coze.NewCozeAPI(authCli,
		coze.WithBaseURL("https://api.coze.cn"),
		coze.WithHttpClient(customClient),
	)
	now := time.Now()
	//
	// Step one, create chats
	req := &coze.CreateChatsReq{
		BotID:  botID,
		UserID: userID,
		Messages: []*coze.Message{
			coze.BuildUserQuestionText("What can you do?", nil),
		},
	}

	resp, err := cozeCli.Chat.Stream(ctx, req)
	if err != nil {
		fmt.Printf("Error starting chats: %v\n", err)
		return
	}

	defer resp.Close()
	for {
		event, err := resp.Recv()
		if errors.Is(err, io.EOF) {
			fmt.Println("Stream finished")
			break
		}
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Println(time.Now().Sub(now).String())
		if event.Event == coze.ChatEventConversationMessageDelta {
			fmt.Print(event.Message.Content)
		} else if event.Event == coze.ChatEventConversationChatCompleted {
			fmt.Printf("Token usage:%d\n", event.Chat.Usage.TokenCount)
		} else {
			fmt.Printf("\n")
		}
	}

	fmt.Printf("done, log:%s\n", resp.Response().LogID())
}
