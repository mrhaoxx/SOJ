package main

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"net/http"
)

// AuthMiddleware
// checks if the user is authenticated by identifying user's unique token
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    0,
				"message": "Token is required",
				"data":    nil,
			})
			c.Abort()
			return
		}

		var user User
		if err := db.Where("token = ?", token).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    0,
				"message": "Invalid Token",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// Save the user in the context
		c.Set("user", user.ID)
		c.Set("is_admin", IsAdmin(user.ID))

		c.Next()
	}
}

// listSubmits
// list submits with limited information
// does not need to be authenticated
func listSubmits(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		c.JSON(400, gin.H{
			"message": "Invalid parameter: page",
		})
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		c.JSON(400, gin.H{
			"message": "Invalid parameter: limit",
		})
		return
	}

	var submits []SubmitCtx
	var total int64
	db.Select("id", "user", "problem", "submit_time", "status", "msg", "judge_result").
		Order("submit_time desc").
		Offset((page - 1) * limit).Limit(limit).
		Find(&submits)
	db.Model(&SubmitCtx{}).Count(&total)

	admin, _ := c.Get("is_admin")
	user, _ := c.Get("user")
	if !admin.(bool) {
		for i := range submits {
			if submits[i].User != user.(string) {
				submits[i].User = "Anonymous"
			}
		}
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":   total,
			"submits": submits,
		},
	})
}

// getSubmitDetail
// return the whole row of a submit in database, including workflow
func getSubmitDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "Invalid parameter",
			"data":    nil,
		})
		return
	}

	submit := SubmitCtx{}
	if err := db.
		Where("id = ?", id).
		First(&submit).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1,
			"message": "Submit not found",
			"data":    nil,
		})
		return
	}

	admin, _ := c.Get("is_admin")
	user, _ := c.Get("user")
	if !admin.(bool) && submit.User != user.(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    1,
			"message": "You are not allowed to view this submit",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    submit,
	})
	return
}

// listRank
// list rank of users
func listRank(c *gin.Context) {
	var users []User
	db.Order("total_score desc").
		Find(&users)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    users,
	})
}

// getUserSummary
func getUserSummary(c *gin.Context) {
	id, _ := c.Get("user")
	var user User
	db.Where("id = ?", id.(string)).
		First(&user)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    user,
	})
	return
}

func serveHTTP(addr string) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	err := router.SetTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to set trusted proxies")
		return
	}

	auth := router.Group("/api/v1", AuthMiddleware())
	auth.GET("rank", listRank)
	auth.GET("list", listSubmits)
	auth.GET("my", getUserSummary)
	auth.GET("status/:id", getSubmitDetail)

	go func() {
		log.Info().Str("addr", addr).Msg("HTTP server started")
		err = router.Run(addr)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to start HTTP server")
		}
	}()
}
