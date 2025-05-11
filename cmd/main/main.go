package main

import (
	"log"
	"net/http"

	"github.com/dev-jiemu/coupon-issuance-poc/pkg/cache"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/gen/v1/v1connect"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/service"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	// 1. 서버 최초 가동 : campaign 관리할 매니저 객체 생성
	cache.NewCampaignManager()

	// 2. service handlers
	campaignServer := service.NewCampaignServer()
	couponServer := service.NewCouponServer()

	// 3. Set up mux and handlers
	mux := http.NewServeMux()

	// Campaign service routes
	campaignPath, campaignHandler := v1connect.NewCampaignServiceHandler(campaignServer)
	mux.Handle(campaignPath, campaignHandler)

	// Coupon service routes
	couponPath, couponHandler := v1connect.NewCouponServiceHandler(couponServer)
	mux.Handle(couponPath, couponHandler)

	// 4. Start server with h2c for HTTP/2 without TLS
	log.Println("RPC server starting on localhost:50051")
	http.ListenAndServe(
		"localhost:50051",
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
