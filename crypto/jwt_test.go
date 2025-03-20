package crypto

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	secret = "bitcoin-to-1-milly"
	issuer = "api.ticketing.io"

	addr = "erd111"
	mail = "test@mail"
)

func Test_test(t *testing.T) {
	bearer := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBZGRyZXNzIjoiMHgwN0Y0NjBjOEM0MWNCZjMwOTQyMkJGQkM2RWZEQkJkNmY0NDE1Mjk4IiwiaXNzIjoibG9jYWxob3N0OjUwMDAiLCJleHAiOjE3MzgyNTk1ODN9.5qNelqYDk2oCTOeRyrN5H7vlNaZdsuu-IVYC5lm-Fhs"
	secret := "a5aa6a0ead4b1c60a6e23ef3a97f8bf1e6d712debd0ecd516acfcfc5d177d1e4"
	claim, err := parseTokenUnverified(bearer, secret)
	require.Nil(t, err)
	fmt.Println(claim)
}

func TestGenerateJwt_ForTestExpiry25(t *testing.T) {
	t.Parallel()

	jwt, _ := GenerateJwt(addr, secret, issuer, 25)
	t.Log(jwt)
}

func TestGenerateJwt_ShouldValidateThenParseClaims(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateJwt(addr, secret, issuer, 5)
	require.Nil(t, err)

	fmt.Println(jwt)

	c, err := ValidateJwt(jwt, secret)
	require.Nil(t, err)
	require.Equal(t, addr, c.Address)
	require.Equal(t, issuer, c.Issuer)
}

func TestGenerateJwt_ShouldNotValidateWrongExpiry(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateJwt(addr, secret, issuer, 0)
	require.Nil(t, err)

	time.Sleep(time.Second * 1)

	c, err := ValidateJwt(jwt, secret)
	require.NotNil(t, err)
	require.True(t, c.Address == "")
}

func TestGenerateJwt_ShouldNotValidateWrongSecret(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateJwt(addr, "bad", issuer, 5)
	require.Nil(t, err)

	c, err := ValidateJwt(jwt, secret)
	require.NotNil(t, err)
	require.True(t, c.Address == "")
}

func TestGetClaims_ShouldReturnForValidToken(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateJwt(addr, secret, issuer, 5)
	require.Nil(t, err)

	c, err := GetClaims(jwt, secret, true)
	require.Nil(t, err)
	require.Equal(t, c.Address, addr)
}

func TestGetClaims_ShouldReturnForExpiredToken(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateJwt(addr, secret, issuer, 0)
	require.Nil(t, err)

	time.Sleep(time.Second * 1)

	c, err := GetClaims(jwt, secret, false)
	require.Nil(t, err)
	require.Equal(t, c.Address, addr)
}

// ################################# Confirm JWT #################################

func TestGenerateConfirmJwt_ShouldValidateThenParseClaims(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateConfirmJwt(addr, mail, secret, issuer, 5)
	require.Nil(t, err)

	fmt.Println(jwt)

	c, err := ValidateJwt(jwt, secret)
	require.Nil(t, err)
	require.Equal(t, addr, c.Address)
	require.Equal(t, issuer, c.Issuer)
}

func TestGenerateConfirmJwt_ShouldNotValidateWrongExpiry(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateConfirmJwt(addr, mail, secret, issuer, 0)
	require.Nil(t, err)

	time.Sleep(time.Second * 1)

	c, err := ValidateConfirmJwt(jwt, secret)
	require.NotNil(t, err)
	require.True(t, c.Address == "")
	require.True(t, c.Email == "")
}

func TestGenerateConfirmJwt_ShouldNotValidateWrongSecret(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateConfirmJwt(addr, mail, "bad", issuer, 5)
	require.Nil(t, err)

	c, err := ValidateConfirmJwt(jwt, secret)
	require.NotNil(t, err)
	require.True(t, c.Address == "")
	require.True(t, c.Email == "")
}

func TestValidateConfirmJwt_ShouldReturnForValidToken(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateConfirmJwt(addr, mail, secret, issuer, 5)
	require.Nil(t, err)

	c, err := ValidateConfirmJwt(jwt, secret)
	require.Nil(t, err)
	require.Equal(t, c.Address, addr)
	require.Equal(t, c.Email, mail)
}

func TestValidateConfirmJwt_ShouldFailForExpiredToken(t *testing.T) {
	t.Parallel()

	jwt, err := GenerateConfirmJwt(addr, mail, secret, issuer, 0)
	require.Nil(t, err)

	time.Sleep(time.Second * 1)

	c, err := ValidateConfirmJwt(jwt, secret)
	require.NotNil(t, err)
	require.True(t, c.Address == "")
	require.True(t, c.Email == "")
}
