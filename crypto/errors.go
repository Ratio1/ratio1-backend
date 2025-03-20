package crypto

import "errors"

var ErrJwtExpired = errors.New("jwt token expired")

var ErrJwtParse = errors.New("jwt parse error")

var ErrInvalidPublicKey = errors.New("invalid public key")

var ErrInvalidPrivateKey = errors.New("invalid private key")

var ErrInvalidSignature = errors.New("invalid signature")
