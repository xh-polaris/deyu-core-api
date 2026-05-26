package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/deyu-core-api/biz/adaptor"
	"github.com/xh-polaris/deyu-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/deyu-core-api/biz/infra/config"
	"github.com/xh-polaris/deyu-core-api/biz/infra/cst"
	invitecode "github.com/xh-polaris/deyu-core-api/biz/infra/mapper/invite_code"
	"github.com/xh-polaris/deyu-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/deyu-core-api/biz/infra/util"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IInviteService interface {
	Check(ctx context.Context, code, phone string) (*InviteCheckResp, error)
	Gen(ctx context.Context, maxCount int) (*InviteGenResp, error)
}

type InviteCheckResp struct {
	Resp   *basic.Response `json:"resp"`
	Token  string          `json:"token,omitempty"`
	UserId string          `json:"user_id,omitempty"`
	Expire int64           `json:"expire,omitempty"`
}

type InviteGenResp struct {
	Resp *basic.Response `json:"resp"`
	Code string          `json:"code,omitempty"`
}

type InviteService struct {
	InviteCodeMapper invitecode.MongoMapper
	UserMapper       user.MongoMapper
	Redis            *redis.Redis
}

var InviteServiceSet = wire.NewSet(
	wire.Struct(new(InviteService), "*"),
	wire.Bind(new(IInviteService), new(*InviteService)),
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateCode(length int) (string, error) {
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

func (s *InviteService) Gen(ctx context.Context, maxCount int) (*InviteGenResp, error) {
	code, err := generateCode(8)
	if err != nil {
		return nil, cst.InviteCodeGenErr
	}

	ic := &invitecode.InviteCode{
		Id:         primitive.NewObjectID(),
		Code:       code,
		MaxCount:   maxCount,
		UsedCount:  0,
		CreateTime: time.Now(),
	}

	if err = s.InviteCodeMapper.InsertOne(ctx, ic); err != nil {
		return nil, cst.InviteCodeGenErr
	}

	return &InviteGenResp{Resp: util.Success(), Code: code}, nil
}

func (s *InviteService) Check(ctx context.Context, code, phone string) (*InviteCheckResp, error) {
	// 校验邀请码
	ic, err := s.InviteCodeMapper.FindByCode(ctx, code)
	if err != nil {
		if errors.Is(err, cst.NotFound) {
			return nil, cst.InviteCodeNotFound
		}
		return nil, err
	}

	// 检查使用次数
	if ic.UsedCount >= ic.MaxCount {
		return nil, cst.InviteCodeUsedUp
	}

	// 查找用户
	u, findErr := s.UserMapper.FindOneByPhone(ctx, phone)
	if findErr != nil && !errors.Is(findErr, cst.NotFound) {
		return nil, findErr
	}

	if !errors.Is(findErr, cst.NotFound) && u.InviteCode != "" {
		// 用户已绑定邀请码
		return nil, cst.UserAlreadyBound
	}

	if errors.Is(findErr, cst.NotFound) {
		// 创建新用户
		u = &user.User{
			Id:         primitive.NewObjectID(),
			Name:       "未命名用户",
			Phone:      phone,
			InviteCode: code,
			CreateTime: time.Now(),
		}
		if err = s.UserMapper.InsertOne(ctx, u); err != nil {
			return nil, cst.LoginErr
		}
	} else {
		// 更新已有用户的邀请码
		u.InviteCode = code
		if err = s.UserMapper.Update(ctx, u); err != nil {
			return nil, cst.LoginErr
		}
	}

	// 原子递增使用次数
	if err = s.InviteCodeMapper.IncrementUsedCount(ctx, ic.Id); err != nil {
		return nil, err
	}

	// 生成 JWT token
	uid := u.Id.Hex()
	secret, expire := config.GetConfig().Auth.SecretKey, config.GetConfig().Auth.AccessExpire
	token, exp, err := adaptor.GenerateJwtToken(uid, secret, expire)
	if err != nil {
		return nil, cst.LoginErr
	}

	return &InviteCheckResp{
		Resp:   util.Success(),
		Token:  token,
		UserId: uid,
		Expire: exp,
	}, nil
}
