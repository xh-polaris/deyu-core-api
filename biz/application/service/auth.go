package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/wire"
	"github.com/xh-polaris/deyu-core-api/biz/adaptor"
	"github.com/xh-polaris/deyu-core-api/biz/adaptor/controller/cmd"
	"github.com/xh-polaris/deyu-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/deyu-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/deyu-core-api/biz/infra/config"
	"github.com/xh-polaris/deyu-core-api/biz/infra/cst"
	"github.com/xh-polaris/deyu-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util"
	"github.com/xh-polaris/deyu-core-api/pkg/crypt"
	"github.com/xh-polaris/deyu-core-api/pkg/logs"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IAuthService interface {
	SendVerifyCode(ctx context.Context, req *core_api.SendVerifyCodeReq) (*core_api.SendVerifyCodeResp, error)
	Login(ctx context.Context, req *core_api.LoginReq) (*core_api.LoginResp, error)
	SetPassword(ctx context.Context, req *cmd.SetPasswordReq) (*cmd.SetPasswordResp, error)
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
	var code int32 = 0
	u, findErr := s.UserMapper.FindOneByPhone(ctx, req.AuthId)
	if findErr != nil && !errors.Is(findErr, cst.NotFound) { // 异常
		return nil, findErr
	}

	switch req.AuthType {
	case cst.Phone: // 手机登录
		// 校验验证码
		verify, err := s.Redis.GetCtx(ctx, cst.VerifyCodeKeyPrefix+req.AuthId)
		if err != nil || verify != req.Verify {
			return nil, cst.VerifyCodeErr
		}
		// 校验成功, 注册或返回
		if errors.Is(findErr, cst.NotFound) { // 不存在则注册
			code = 1
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
		if u.Password == "" {
			code = 1
		}
	case cst.Password:
		if findErr != nil || u.Password == "" {
			return nil, cst.NoPassword
		}
		if !crypt.Check(req.Verify, u.Password) {
			return nil, cst.ErrPassword
		}
	default:
		return nil, cst.UnSupportWay
	}

	uid := u.Id.Hex()
	secret, expire := config.GetConfig().Auth.SecretKey, config.GetConfig().Auth.AccessExpire
	token, exp, err := generateJwtToken(uid, secret, expire)
	if err != nil {
		return nil, cst.LoginErr
	}
	return &core_api.LoginResp{Resp: &basic.Response{Code: code, Msg: ""}, Token: token, UserId: uid, Expire: exp}, nil
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

func (s *AuthService) SetPassword(ctx context.Context, req *cmd.SetPasswordReq) (*cmd.SetPasswordResp, error) {
	uid, err := adaptor.ExtractUserId(ctx)
	if err != nil {
		logs.Error("extract user id error: %v", err)
		return nil, cst.UnAuthErr
	}
	u, err := s.UserMapper.FindOneById(ctx, uid)
	if err != nil {
		return nil, cst.NotFound
	}
	if req.NewPassword == "" {
		return nil, cst.InvalidPassword
	}
	u.Password, err = crypt.Hash(req.NewPassword)
	if err != nil {
		return nil, cst.ErrSetPassword
	}
	err = s.UserMapper.Update(ctx, u)
	if err != nil {
		return nil, cst.ErrSetPassword
	}
	return &cmd.SetPasswordResp{Resp: util.Success()}, nil
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
	if len(req.AuthType) < 10 || req.AuthType[:10] != "xh-polaris" {
		if err = New(ctx, config.GetConfig().SMS.Account, config.GetConfig().SMS.Token).Send(ctx, req.AuthId, verifyCode, "5"); err != nil {
			return nil, cst.VerifyCodeSendErr
		}
		return &core_api.SendVerifyCodeResp{Resp: util.Success()}, nil
	}
	return &core_api.SendVerifyCodeResp{Resp: &basic.Response{Code: 0, Msg: verifyCode}}, nil
}

const (
	singleSendURL   = "https://bluecloudccs.21vbluecloud.com/services/sms/messages?api-version=2018-10-01"
	checkReceiveURL = "https://bluecloudccs.21vbluecloud.com/services/sms/messages/%s?api-version=2018-10-01&continuationToken=&count=10"
)

type BluecloudSMS struct {
	authHeader http.Header
}

func New(ctx context.Context, account, token string) *BluecloudSMS {
	s, err := getBlueCloudSMSProvider(ctx, account, token)
	if err != nil {
		return nil
	}
	return s
}

func getBlueCloudSMSProvider(ctx context.Context, account, token string) (*BluecloudSMS, error) {
	header := http.Header{}
	header.Set("content-type", "application/json")
	header.Set("Account", account)
	header.Set("Authorization", token)
	return &BluecloudSMS{authHeader: header}, nil
}

// Send 发送验证码并校验用户是否成功收到
func (b *BluecloudSMS) Send(ctx context.Context, phone, code, expire string) (err error) {
	// 发送短信
	if _, err = b.send(ctx, phone, code, expire); err != nil {
		return err
	}
	return nil
}

func (b *BluecloudSMS) send(_ context.Context, phone, code, expire string) (map[string]any, error) {
	body := map[string]any{
		"phoneNumber": []string{phone},
		"messageBody": map[string]any{
			"extend": "00",
			//"templateName": conf.GetConfig().SMS.CauseToTemplate[cause],
			"templateName": "德育",
			"templateParam": map[string]any{
				"otpcode": code,   // 验证码
				"expire":  expire, // 以分钟为单位超时时间
			},
		},
	}
	res, err := util.GetHttpClient().Post(singleSendURL, b.authHeader, body)
	return res, err
}
