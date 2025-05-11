package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

var (
	chosung  = []rune{'ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'}
	jungsung = []rune{'ㅏ', 'ㅐ', 'ㅑ', 'ㅒ', 'ㅓ', 'ㅔ', 'ㅕ', 'ㅖ', 'ㅗ', 'ㅘ', 'ㅙ', 'ㅚ', 'ㅛ', 'ㅜ', 'ㅝ', 'ㅞ', 'ㅟ', 'ㅠ', 'ㅡ', 'ㅢ', 'ㅣ'}
)

// 완성된 한글 문자 생성
func getRandomKoreanChar() (rune, error) {
	cho, err := randomIndex(len(chosung))
	if err != nil {
		return 0, err
	}

	jung, err := randomIndex(len(jungsung))
	if err != nil {
		return 0, err
	}

	unicodeChar := 0xAC00 + (cho * 21 * 28) + (jung * 28)
	return rune(unicodeChar), nil
}

// 랜덤 인덱스 생성
func randomIndex(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func getRandomCharOrDigit() (rune, error) {
	// 0: 한글, 1: 숫자
	t, err := rand.Int(rand.Reader, big.NewInt(5))
	if err != nil {
		return 0, err
	}

	// 80% 확률로 한글, 20% 확률로 숫자 선택
	if t.Int64() < 4 {
		return getRandomKoreanChar()
	} else {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return 0, err
		}
		return rune('0' + d.Int64()), nil
	}
}

func GenerateCouponCode(length int) (string, error) {
	if length <= 0 || length > 10 {
		// 길이 제한 확인
		length = 10
	}

	var sb strings.Builder

	// 현재 시간을 기반으로 프리픽스 추가
	timestamp := time.Now().UnixNano()
	if length >= 6 {
		prefix := fmt.Sprintf("%03d", timestamp%1000)
		sb.WriteString(prefix)
	}

	// 남은 문자 생성
	remainingLength := length - sb.Len()
	for i := 0; i < remainingLength; i++ {
		char, err := getRandomCharOrDigit()
		if err != nil {
			return "", err
		}
		sb.WriteRune(char)
	}

	return sb.String(), nil
}
