package identity

import (
	"github.com/golang-jwt/jwt/v5"
)

type Identity interface {
	MarshalPublic() ([]byte, error)
	Sign(claims jwt.Claims) (string, error)
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}
