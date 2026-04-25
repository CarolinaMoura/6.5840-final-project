package main

import "math/rand"

// Returns whether or not a byte represents an alphanumeric character.
func isAlphaNumericCharacter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// Returns whether or not a string consists of only alphanumeric characters.
func isAlphaNumericString(s string) bool {
	for _, c := range s {
		if !isAlphaNumericCharacter(byte(c)) {
			return false
		}
	}
	return true
}

// Generates a random alphanumeric string consisting of uppercase letters and numbers.
func generateAlphanumericString(length int) string {
	str := []byte{}
	for i := 0; i < length; i++ {
		str = append(str, 'A'+byte(rand.Intn(26)))
	}
	return string(str)
}
