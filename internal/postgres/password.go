package postgres

import (
	"crypto/rand"
	"math/big"
)

const passwordAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GeneratePassword produit un mot de passe sûr de la longueur souhaitée.
func GeneratePassword(length int) (string, error) {
	if length <= 0 {
		length = 24
	}

	var runes = []rune(passwordAlphabet)
	result := make([]rune, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(runes))))
		if err != nil {
			return "", err
		}
		result[i] = runes[n.Int64()]
	}
	return string(result), nil
}
