//internal/auth/jwt.go

package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)


type Claims struct {
	UserID uuid.UUID    `json:"user_id"`
	jwt.RegisteredClaims
}
//registeredClaims is a struct provided by the JWT library (github.com/golang-jwt/jwt/v5) that contains the standard JWT fields defined in the JWT specification

type JWTService struct {
	secret []byte
}


func NewJWTService(secret  string) *JWTService{
	return &JWTService{
		secret: []byte(secret),
	}  
}  //so that we dont have to write secret for every function which is going to come below : generateToken, validateToken, 

//token generator -> takes in userID and embed it into the jwt object , along with standard jwt data which comes naturally under RegsiteredClaims
func (j  *JWTService) GenerateToken(userID uuid.UUID) (string, error){
	claims := Claims{
		UserID: userID, 
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24*time.Hour)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},   //this destrctures the object into one thing, if we did not make claims as a struct then the final thing would have had become {x, y, z , registeredClaims:{}} and then jwt wouldnt be able to recognise the payload 
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(j.secret)  //return the signed jwt string 


}



func (j *JWTService) ValidateToken(tokenString string) (*Claims, error){
	token, err := jwt.ParseWithClaims(
		tokenString, 
		&Claims{},
		func(token *jwt.Token) (interface{}, error){
			return j.secret, nil
		},
	)

	if err != nil {
		return nil, err
	}
	//else:
	claims,ok:= token.Claims.(*Claims)
	if !ok || !token.Valid{
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil

}


