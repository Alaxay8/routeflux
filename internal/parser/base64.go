package parser

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func decodeBase64Payload(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	compact := strings.NewReplacer("\n", "", "\r", "", "\t", "", " ", "").Replace(trimmed)

	candidates := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	}

	for _, decode := range candidates {
		data, err := decode(compact)
		if err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("unsupported base64 payload")
}
