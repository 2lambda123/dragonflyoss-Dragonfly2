package middlewares

import (
	"time"

	"d7y.io/dragonfly/v2/manager/service"
	"d7y.io/dragonfly/v2/manager/types"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type user struct {
	ID uint
}

func Jwt(service service.REST) (*jwt.GinJWTMiddleware, error) {
	var identityKey = "id"
	authMiddleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "Dragonfly",
		Key:         []byte("Secret Key"),
		Timeout:     time.Hour,
		MaxRefresh:  time.Hour,
		IdentityKey: identityKey,

		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &user{
				ID: claims[identityKey].(uint),
			}
		},

		Authenticator: func(c *gin.Context) (interface{}, error) {
			var json types.SignInRequest
			if err := c.ShouldBind(&json); err != nil {
				return "", jwt.ErrMissingLoginValues
			}

			u, err := service.SignIn(json)
			if err != nil {
				return "", jwt.ErrFailedAuthentication
			}

			return &user{
				ID: u.ID,
			}, nil
		},

		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if u, ok := data.(user); ok {
				return jwt.MapClaims{
					identityKey: u.ID,
				}
			}
			return jwt.MapClaims{}
		},

		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"message": message,
			})
		},

		LoginResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			c.JSON(code, gin.H{
				"token":  token,
				"expire": expire.Format(time.RFC3339),
			})
		},

		LogoutResponse: func(c *gin.Context, code int) {
			c.Status(code)
		},

		RefreshResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			c.JSON(code, gin.H{
				"token":  token,
				"expire": expire.Format(time.RFC3339),
			})
		},

		TokenLookup:    "header: Authorization, query: token, cookie: jwt",
		TokenHeadName:  "Bearer",
		TimeFunc:       time.Now,
		SendCookie:     true,
		CookieHTTPOnly: true,
	})

	if err != nil {
		return nil, err
	}

	return authMiddleware, nil
}
