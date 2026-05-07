// Package signing implements CCH (Client Crypto Hash) signing for Claude API requests.
// Claude Code computes cch on every request; missing or invalid cch is a detectable
// fingerprint that Anthropic uses to identify third-party clients.
//
// Algorithm (from reversing Claude Code):
// 1. Find the billing header in system[0].text
// 2. Zero out cch value (cch=XXXXX -> cch=00000)
// 3. Compute xxHash64 of the modified body with seed 0x6E52736AC806831E
// 4. Take lower 20 bits → format as 5 hex digits
// 5. Replace cch=00000 with the computed hash
package signing

import (
	"fmt"
	"regexp"
	"strings"

	xxHash64 "github.com/pierrec/xxHash/xxHash64"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Seed is the xxHash64 seed used by Claude Code for CCH computation.
const Seed uint64 = 0x6E52736AC806831E

// cchPattern matches cch=XXXXX in the billing header.
var cchPattern = regexp.MustCompile(`\bcch=([0-9a-f]{5});`)

// SignBody computes and injects the CCH signature into the request body.
// If the body doesn't contain a cch placeholder, it returns the body unchanged.
func SignBody(body []byte) []byte {
	// Extract billing header from system prompt
	billingHeader := gjson.GetBytes(body, "system.0.text").String()
	if !strings.HasPrefix(billingHeader, "x-anthropic-billing-header:") {
		return body
	}
	if !cchPattern.MatchString(billingHeader) {
		return body
	}

	// Zero out CCH for computation
	unsignedBillingHeader := cchPattern.ReplaceAllString(billingHeader, "cch=00000;")
	unsignedBody, err := sjson.SetBytes(body, "system.0.text", unsignedBillingHeader)
	if err != nil {
		return body
	}

	// Compute CCH: lower 20 bits of xxHash64
	cch := computeCCH(unsignedBody)
	signedBillingHeader := cchPattern.ReplaceAllString(unsignedBillingHeader, "cch="+cch+";")
	signedBody, err := sjson.SetBytes(unsignedBody, "system.0.text", signedBillingHeader)
	if err != nil {
		return unsignedBody
	}
	return signedBody
}

// computeCCH computes the xxHash64 checksum and formats as 5 hex digits.
func computeCCH(data []byte) string {
	hash := xxHash64.New(Seed)
	hash.Write(data)
	return fmt.Sprintf("%05x", hash.Sum64()&0xFFFFF)
}
