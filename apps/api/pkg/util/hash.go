package util

import (
	"crypto/md5"
	"encoding/hex"
	"strings"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// HashMailboxKey creates an MD5 hash from the mailbox name and raw address, used for change detection.
func HashMailboxKey(name string, addr model.AddressRaw) string {
	builder := strings.Builder{}
	builder.WriteString(strings.TrimSpace(strings.ToLower(name)))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(strings.ToLower(addr.Street)))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(strings.ToLower(addr.City)))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(strings.ToLower(addr.State)))
	builder.WriteString("|")
	builder.WriteString(strings.TrimSpace(strings.ToLower(addr.Zip)))
	return hashString(builder.String())
}

// HashString returns the MD5 hash of an arbitrary string.
func HashString(input string) string {
	return hashString(strings.TrimSpace(strings.ToLower(input)))
}

func hashString(input string) string {
	sum := md5.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}
