package user

import (
	"context"
	"errors"

	"github.com/xh-polaris/deyu-core-api/biz/infra/config"
	"github.com/xh-polaris/deyu-core-api/biz/infra/cst"
	"github.com/zeromicro/go-zero/core/stores/monc"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var _ MongoMapper = (*mongoMapper)(nil)

const (
	collection     = "user"
	cacheKeyPrefix = "cache:user:"
)

type MongoMapper interface {
	FindOneByPhone(ctx context.Context, phone string) (*User, error)
	InsertOne(ctx context.Context, user *User) error
}

type mongoMapper struct {
	conn *monc.Model
}

func NewUserMongoMapper(config *config.Config) MongoMapper {
	conn := monc.MustNewModel(config.Mongo.URL, config.Mongo.DB, collection, config.Cache)
	return &mongoMapper{conn: conn}
}

func (m *mongoMapper) FindOneByPhone(ctx context.Context, phone string) (*User, error) {
	var u User
	err := m.conn.FindOneNoCache(ctx, &u, bson.M{cst.Phone: phone})

	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, cst.NotFound
	case err == nil:
		return &u, nil
	}
	return nil, err
}

func (m *mongoMapper) InsertOne(ctx context.Context, user *User) error {
	_, err := m.conn.InsertOneNoCache(ctx, user)
	return err
}
