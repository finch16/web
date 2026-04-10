package main

import "crypto/sha256"
import "encoding/hex"

func sha256Hex(password string) string {
    hash := sha256.Sum256([]byte(password))
    return hex.EncodeToString(hash[:])
}
