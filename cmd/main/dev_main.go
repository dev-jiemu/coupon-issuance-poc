package main

import (
	"fmt"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/utils"
)

// 한글+숫자 10 length test
func main() {
	// 10개의 고유한 쿠폰 코드 생성
	for i := 0; i < 10; i++ {
		code, err := utils.GenerateCouponCode(10)
		if err != nil {
			fmt.Printf("오류 발생: %v\n", err)
			continue
		}

		fmt.Printf("쿠폰 코드 %d: %s\n", i+1, code)
	}
}
