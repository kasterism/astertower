package utils

import (
	"fmt"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	randomLength           = 5
	maxNameLength          = 63
	maxGeneratedNameLength = maxNameLength - randomLength
)

func GenerateName(base string) string {
	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, utilrand.String(randomLength))
}
