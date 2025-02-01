package utils

import "strings"

func ExtractNetAddress(apiAddress string) (string, string) {
	idx := strings.Index(apiAddress, "://")
	if idx < 0 {
		return "tcp", apiAddress
	}

	return apiAddress[:idx], apiAddress[idx+3:]
}
