package user

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	Id         primitive.ObjectID `json:"id" bson:"_id"`                  // id
	Name       string             `json:"name" bson:"name"`               // 用户名
	Phone      string             `json:"phone" bson:"phone"`             // 手机号
	Password   string             `json:"password" bson:"password"`       // 密码
	CreateTime time.Time          `json:"create_time" bson:"create_time"` // 时间
}
