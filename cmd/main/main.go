package main

import (
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/cache"
)

func main() {
	// TODO :
	// 쿠폰 발급할 캠페인 생성해야함
	// 생성할때 발급 가능한 쿠폰 수, 쿠폰 발급이 시작되는 날짜, expired 지정

	// 캠페인 생성 시점에 쿠폰 수 만큼 사용기간 range 정해서 만들어두고, 나중에 시간 지나고 나서 요청중으로 발급유무 체크에 엽데이트 하는 방향으로 가야할듯
	// 그래서 캠페인 시작 '이전' 에 발급요청하면 막고, 캠페인 종료 이후에 발급요청해도 막아야 함

	// 1. 서버 최초 가동 : campaign 관리할 매니저 객체 생성
	cache.NewCampaignManager()

	// TODO : 2. RPC Server Start
}
