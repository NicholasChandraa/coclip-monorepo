package password

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultCost is the bcrypt cost factor (minimum 12 for production)
	DefaultCost = 12
)

// Hash creates a bcrypt hash of the password
func Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	return string(bytes), err
}

// Verify compares a password with a hash
func Verify(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// NeedsRehash checks if the hash needs to be updated (different cost)
func NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return true
	}
	return cost < DefaultCost
}
