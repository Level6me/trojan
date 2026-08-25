package web

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"trojan/core"
	"trojan/util"
	"trojan/web/controller"
)

var (
	identityKey    = "id"
	authMiddleware *jwt.GinJWTMiddleware
	err            error
)

// Login auth用户验证结构体
type Login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

func getSecretKey() string {
	sk, _ := core.GetValue("secretKey")
	if sk == "" {
		sk = util.RandString(15, util.ALL)
		core.SetValue("secretKey", sk)
	}
	return sk
}

func jwtInit(timeout int) {
	authMiddleware, err = jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "trojan-manager",
		Key:         []byte(getSecretKey()),
		Timeout:     time.Minute * time.Duration(timeout),
		MaxRefresh:  time.Minute * time.Duration(timeout),
		IdentityKey: identityKey,
		SendCookie:  true,
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*Login); ok {
				return jwt.MapClaims{
					identityKey: v.Username,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &Login{
				Username: claims[identityKey].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var (
				password  string
				loginVals Login
			)
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			userID := loginVals.Username
			pass := loginVals.Password

			encryPass := fmt.Sprintf("%x", sha256.Sum224([]byte(pass)))

			if userID != "admin" {
				mysql := core.GetMysql()
				user := mysql.GetUserByName(userID)
				if user == nil {
					return nil, jwt.ErrFailedAuthentication
				}
				if user.EncryptPass == encryPass || user.Password == pass || user.EncryptPass == pass {
					return &loginVals, nil
				}
			} else {
				password, err = core.GetValue(userID + "_pass")
				if err != nil || password == "" {
					return nil, jwt.ErrFailedAuthentication
				}
				if password == pass || password == encryPass {
					return &loginVals, nil
				}
			}
			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			if _, ok := data.(*Login); ok {
				return true
			}
			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		TokenLookup:   "header: Authorization, query: token, cookie: jwt",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	})

	if err != nil {
		fmt.Println("JWT Error:" + err.Error())
	}
}

// handlePublicRegister 首次初始化管理员密码（仅在未设置密码时允许一次性初始化）
func handlePublicRegister(c *gin.Context) {
	responseBody := controller.ResponseBody{Msg: "success"}
	defer controller.TimeCost(time.Now(), &responseBody)

	// 安全检查：如果已存在 admin 密码，坚决拒绝未授权重置
	adminPass, _ := core.GetValue("admin_pass")
	if adminPass != "" {
		c.JSON(403, gin.H{
			"code":    403,
			"message": "管理员密码已初始化，公开注册通道已关闭！若忘记密码请登录控制台或通过服务器命令行 trojan pass [新密码] 重置。",
			"Msg":     "forbidden",
		})
		return
	}

	pass := strings.TrimSpace(c.PostForm("password"))
	if pass == "" {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "初始管理员密码不能为空！",
			"Msg":     "password cannot be empty",
		})
		return
	}

	encryPass := fmt.Sprintf("%x", sha256.Sum224([]byte(pass)))
	err := core.SetValue("admin_pass", encryPass)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	c.JSON(200, responseBody)
}

// handleAuthResetPass 登录后安全修改管理员密码
func handleAuthResetPass(c *gin.Context) {
	responseBody := controller.ResponseBody{Msg: "success"}
	defer controller.TimeCost(time.Now(), &responseBody)

	username := RequestUsername(c)
	if username != "admin" {
		c.JSON(403, gin.H{"code": 403, "message": "非管理员用户无权执行此操作", "Msg": "forbidden"})
		return
	}

	pass := strings.TrimSpace(c.PostForm("password"))
	if pass == "" {
		c.JSON(400, gin.H{"code": 400, "message": "新密码不能为空", "Msg": "password cannot be empty"})
		return
	}

	encryPass := fmt.Sprintf("%x", sha256.Sum224([]byte(pass)))
	err := core.SetValue("admin_pass", encryPass)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	c.JSON(200, responseBody)
}

// RequestUsername 获取请求接口的用户名
func RequestUsername(c *gin.Context) string {
	claims := jwt.ExtractClaims(c)
	return claims[identityKey].(string)
}

// Auth 权限router
func Auth(r *gin.Engine, timeout int) *jwt.GinJWTMiddleware {
	jwtInit(timeout)

	newInstall := gin.H{"code": 201, "message": "No administrator account found inside the database", "data": nil}
	r.NoRoute(authMiddleware.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		fmt.Printf("NoRoute claims: %#v\n", claims)
		c.JSON(404, gin.H{"code": 404, "message": "Page not found"})
	})
	r.GET("/auth/check", func(c *gin.Context) {
		result, err := core.GetValue("admin_pass")
		if err != nil && result == "" {
			c.JSON(500, gin.H{"code": 500, "message": "系统安全存储读取中，请刷新重试"})
			return
		}
		if result == "" {
			c.JSON(201, newInstall)
		} else {
			title, err := core.GetValue("login_title")
			if err != nil || title == "" {
				title = "trojan 管理平台"
			}
			c.JSON(200, gin.H{
				"code":    200,
				"message": "success",
				"data": map[string]string{
					"title": title,
				},
			})
		}
	})
	r.POST("/auth/login", authMiddleware.LoginHandler)
	r.POST("/login", authMiddleware.LoginHandler)
	// 仅在未初始化密码时允许 handlePublicRegister，已初始化则返回 403 Forbidden 彻底杜绝密码被篡改/重置
	r.POST("/auth/register", handlePublicRegister)

	authO := r.Group("/auth")
	authO.Use(authMiddleware.MiddlewareFunc())
	{
		authO.GET("/loginUser", func(c *gin.Context) {
			result, _ := core.GetValue("admin_pass")
			if result == "" {
				c.JSON(201, newInstall)
			} else {
				c.JSON(200, gin.H{
					"code":    200,
					"message": "success",
					"data": map[string]string{
						"username": RequestUsername(c),
					},
				})
			}
		})
		authO.POST("/reset_pass", handleAuthResetPass)
		authO.POST("/logout", authMiddleware.LogoutHandler)
		authO.POST("/refresh_token", authMiddleware.RefreshHandler)
	}
	return authMiddleware
}
