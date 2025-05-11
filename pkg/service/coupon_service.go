package service

import (
	"context"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/cache"
	"log"

	"connectrpc.com/connect"
	v1 "github.com/dev-jiemu/coupon-issuance-poc/pkg/gen/v1"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/gen/v1/v1connect"
)

type CouponServer struct{}

// NewCouponServer creates a new coupon server
func NewCouponServer() v1connect.CouponServiceHandler {
	return &CouponServer{}
}

// IssueCoupon implements the IssueCoupon RPC
func (s *CouponServer) IssueCoupon(context context.Context, req *connect.Request[v1.IssueCouponReq]) (*connect.Response[v1.IssueCouponRes], error) {
	log.Printf("IssueCoupon called with campaignId: %s", req.Msg.CampaignId)

	couponRes := &v1.IssueCouponRes{
		Result: &v1.BaseResponse{
			Success: true,
			Message: "",
		},
	}

	// 쿠폰 발행 요청
	coupon, err := cache.Manager.PublishCoupon(req.Msg.CampaignId)
	if err != nil {
		couponRes.Result.Success = false
		couponRes.Result.Message = err.Error()
	} else {
		couponRes.CouponCode = coupon.CouponId
	}

	return connect.NewResponse(couponRes), nil
}
