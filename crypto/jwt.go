package crypto

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type JwtClaims struct {
	Address string
	jwt.RegisteredClaims
}

type MailConfirmClaims struct {
	Address string
	Email   string
	jwt.RegisteredClaims
}

var isExpired = func(claims JwtClaims) bool {
	return claims.ExpiresAt.Unix() < time.Now().Unix()
}

func GenerateJwt(address, secret, issuer string, minsToExpiration int) (string, error) {
	claims := JwtClaims{
		Address: address,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(minsToExpiration))),
			Issuer:    issuer,
		},
	}
	payload := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)

	return payload.SignedString([]byte(secret))
}

func GenerateConfirmJwt(address, email, secret, issuer string, minsToExpiration int) (string, error) {
	claims := MailConfirmClaims{
		Address: address,
		Email:   email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(minsToExpiration))),
			Issuer:    issuer,
		},
	}
	payload := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	return payload.SignedString([]byte(secret))
}

func ValidateJwt(signedToken, secret string) (JwtClaims, error) {
	claims, err := parseToken(signedToken, secret)
	if err != nil {
		return JwtClaims{}, err
	}

	if isExpired(*claims) {
		return JwtClaims{}, ErrJwtExpired
	}

	return *claims, nil
}

func ValidateConfirmJwt(signedToken, secret string) (MailConfirmClaims, error) {
	token, err := jwt.ParseWithClaims(
		signedToken,
		&MailConfirmClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
	)
	if err != nil {
		return MailConfirmClaims{}, err
	}
	err = token.Claims.Valid()
	if err != nil {
		return MailConfirmClaims{}, err
	}
	claims, ok := token.Claims.(*MailConfirmClaims)
	if !ok {
		return MailConfirmClaims{}, ErrJwtParse
	}
	return *claims, nil
}

func GetClaims(signedToken, secret string, verify bool) (JwtClaims, error) {
	var claims *JwtClaims
	var err error
	if !verify {
		claims, err = parseTokenUnverified(signedToken, secret)
	} else {
		claims, err = parseToken(signedToken, secret)
	}

	if err != nil {
		return JwtClaims{}, errors.New("error while parsing token: " + err.Error())
	}

	return *claims, nil
}

func parseToken(signedToken, secret string) (*JwtClaims, error) {
	token, err := jwt.ParseWithClaims(
		signedToken,
		&JwtClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
	)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JwtClaims)
	if !ok {
		return nil, ErrJwtParse
	}

	return claims, nil
}

func parseTokenUnverified(signedToken, secret string) (*JwtClaims, error) {
	token, err := jwt.ParseWithClaims(
		signedToken,
		&JwtClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
	)

	if err != nil {
		if validationErr, ok := err.(*jwt.ValidationError); ok {
			if validationErr.Errors&(jwt.ValidationErrorExpired) != 0 && token != nil {
				claims, okCast := token.Claims.(*JwtClaims)
				if !okCast {
					return nil, ErrJwtParse
				}

				return claims, nil
			}
		}

		return nil, err
	}

	claims, okCast := token.Claims.(*JwtClaims)
	if !okCast {
		return nil, ErrJwtParse
	}

	return claims, nil
}
