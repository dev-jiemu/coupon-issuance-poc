package models

import (
	"time"
)

type Coupon struct {
	CouponId    string
	StartDate   time.Time
	ExpiredDate time.Time
	PublishYn   bool // 발행여부
	UseYn       bool // 사용여부
}
