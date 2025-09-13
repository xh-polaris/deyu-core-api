package service

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/wire"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/xh-polaris/deyu-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/deyu-core-api/biz/infra/config"
	"github.com/xh-polaris/deyu-core-api/biz/infra/cst"
	"github.com/xh-polaris/deyu-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IAuthService interface {
	SendVerifyCode(ctx context.Context, req *core_api.SendVerifyCodeReq) (*core_api.SendVerifyCodeResp, error)
	Login(ctx context.Context, req *core_api.LoginReq) (*core_api.LoginResp, error)
}

type AuthService struct {
	UserMapper user.MongoMapper
	Redis      *redis.Redis
}

var AuthServiceSet = wire.NewSet(
	wire.Struct(new(AuthService), "*"),
	wire.Bind(new(IAuthService), new(*AuthService)),
)

func (s *AuthService) Login(ctx context.Context, req *core_api.LoginReq) (*core_api.LoginResp, error) {
	// 查找用户
	switch req.AuthType {
	case cst.Phone: // 手机登录
		u, findErr := s.UserMapper.FindOneByPhone(ctx, req.AuthId)
		if findErr != nil && !errors.Is(findErr, cst.NotFound) { // 异常
			return nil, findErr
		}
		// 校验验证码
		verify, err := s.Redis.GetCtx(ctx, cst.VerifyCodeKeyPrefix+req.AuthId)
		if err != nil || verify != req.Verify {
			return nil, cst.VerifyCodeErr
		}
		// 校验成功, 注册或返回
		if errors.Is(findErr, cst.NotFound) { // 不存在则注册
			u = &user.User{
				Id:         primitive.NewObjectID(),
				Name:       "未命名用户",
				Phone:      req.AuthId,
				CreateTime: time.Now(),
			}
			if err = s.UserMapper.InsertOne(ctx, u); err != nil {
				return nil, cst.LoginErr
			}
		}
		uid := u.Id.Hex()
		secret, expire := config.GetConfig().Auth.SecretKey, config.GetConfig().Auth.AccessExpire
		token, exp, err := generateJwtToken(uid, secret, expire)
		if err != nil {
			return nil, cst.LoginErr
		}
		return &core_api.LoginResp{Resp: util.Success(), Token: token, UserId: uid, Expire: exp}, nil
	default:
		return nil, cst.UnSupportWay
	}
}

func generateJwtToken(uid, secret string, expire int64) (string, int64, error) {
	key, err := jwt.ParseECPrivateKeyFromPEM([]byte(secret))
	if err != nil {
		return "", 0, err
	}
	iat := time.Now().Unix()
	exp := iat + expire
	claims := jwt.MapClaims{"exp": exp, "iat": iat, "userId": uid}
	token := jwt.New(jwt.SigningMethodES256)
	token.Claims = claims
	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", 0, err
	}
	return tokenString, exp, nil
}

func (s *AuthService) SendVerifyCode(ctx context.Context, req *core_api.SendVerifyCodeReq) (*core_api.SendVerifyCodeResp, error) {
	if req.AuthId == "" {
		return nil, cst.PhoneNilErr
	}
	// 构造验证码
	code, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return nil, err
	}
	code = code.Add(code, big.NewInt(100000))
	verifyCode := code.String()
	// 缓存验证码
	if err = s.Redis.SetexCtx(ctx, cst.VerifyCodeKeyPrefix+req.AuthId, verifyCode, 300); err != nil {
		return nil, cst.VerifyCodeSendErr
	}
	// 发送验证码
	if err = callSMS(config.GetConfig().SMS, []string{req.AuthId}, code.String()); err != nil {
		return nil, cst.VerifyCodeSendErr
	}
	return &core_api.SendVerifyCodeResp{Resp: util.Success()}, nil
}

func callSMS(sms *config.SMSConfig, phones []string, code string) error {
	// 实例化一个认证对象，入参需要传入腾讯云账户 SecretId 和 SecretKey，此处还需注意密钥对的保密
	// 密钥可前往官网控制台 https://console.cloud.tencent.com/cam/capi 进行获取
	credential := common.NewCredential(sms.SecretId, sms.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint, cpf.HttpProfile.ReqMethod = sms.Host, "POST"
	client := common.NewCommonClient(credential, sms.Region, cpf).WithLogger(log.Default())

	request := tchttp.NewCommonRequest("sms", sms.Version, sms.Action)

	// 模板参数
	codes := []string{code, "5"}
	params := map[string]any{
		"PhoneNumberSet":   phones,
		"SmsSdkAppId":      sms.SmsSdkAppId,
		"TemplateId":       sms.TemplateId,
		"SignName":         sms.SignName,
		"TemplateParamSet": codes,
	}
	if err := request.SetActionParameters(params); err != nil {
		return err
	}

	response := tchttp.NewCommonResponse()
	if err := client.Send(request, response); err != nil {
		logx.Error("fail to invoke api:", err.Error())
		return err
	}
	logx.Info(string(response.GetBody()))
	return nil
}
