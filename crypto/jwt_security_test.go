package crypto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetClaimsRejectsExpiredTokenWithInvalidSignature(t *testing.T) {
	// Regression coverage for GO-2024-3250: ignoring expiry must never ignore
	// a signature error reported by the JWT dependency.
	token, err := GenerateJwt("test-address", "attacker-test-secret", "test-issuer", -1)
	require.NoError(t, err)
	_, err = GetClaims(token, "server-test-secret", false)
	require.Error(t, err)
}
