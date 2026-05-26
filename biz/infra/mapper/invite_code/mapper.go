package invite_code

import (
	"context"
	"errors"

	"github.com/xh-polaris/deyu-core-api/biz/infra/config"
	"github.com/xh-polaris/deyu-core-api/biz/infra/cst"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var _ MongoMapper = (*mongoMapper)(nil)

const (
	collection     = "invite_code"
	cacheKeyPrefix = "cache:invite_code:"
)

type MongoMapper interface {
	FindByCode(ctx context.Context, code string) (*InviteCode, error)
	InsertOne(ctx context.Context, ic *InviteCode) error
	IncrementUsedCount(ctx context.Context, id primitive.ObjectID) error
}

type mongoMapper struct {
	conn *monc.Model
}

func NewInviteCodeMongoMapper(config *config.Config) MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.Cache)
	return &mongoMapper{conn: conn}
}

func (m *mongoMapper) FindByCode(ctx context.Context, code string) (*InviteCode, error) {
	var ic InviteCode
	err := m.conn.FindOneNoCache(ctx, &ic, bson.M{"code": code})

	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, cst.NotFound
	case err == nil:
		return &ic, nil
	}
	return nil, err
}

func (m *mongoMapper) InsertOne(ctx context.Context, ic *InviteCode) error {
	_, err := m.conn.InsertOneNoCache(ctx, ic)
	return err
}

func (m *mongoMapper) IncrementUsedCount(ctx context.Context, id primitive.ObjectID) error {
	_, err := m.conn.UpdateOneNoCache(ctx, bson.M{
		"_id": id,
	}, bson.M{
		"$inc": bson.M{"used_count": 1},
	})
	return err
}
