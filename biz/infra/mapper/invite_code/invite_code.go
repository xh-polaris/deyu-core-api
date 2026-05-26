package invite_code

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type InviteCode struct {
	Id         primitive.ObjectID `json:"id" bson:"_id"`
	Code       string             `json:"code" bson:"code"`
	MaxCount   int                `json:"max_count" bson:"max_count"`
	UsedCount  int                `json:"used_count" bson:"used_count"`
	CreateTime time.Time          `json:"create_time" bson:"create_time"`
}
