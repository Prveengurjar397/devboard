package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserClaims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

func jwtSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return nil
	}
	return []byte(secret)
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.SplitN(c.GetHeader("Authorization"), " ", 2)

		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"error": "authentication required"})
			return
		}

		secret := jwtSecret()
		if secret == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{"error": "authentication unavailable"})
			return
		}

		claims := &UserClaims{}
		token, err := jwt.ParseWithClaims(
			header[1],
			claims,
			func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrSignatureInvalid
				}
				return secret, nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
		)

		if err != nil || !token.Valid || claims.UserID <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

func currentUserID(c *gin.Context) int {
	id, _ := c.Get("user_id")
	userID, _ := id.(int)
	return userID
}

func register(c *gin.Context) {
        var body struct {
                Name        string `json:"name"`
                CompanyName string `json:"company_name"`
                Email       string `json:"email"`
                Password    string `json:"password"`
        }

        if err := c.ShouldBindJSON(&body); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
                return
        }

        body.Name = strings.TrimSpace(body.Name)
        body.CompanyName = strings.TrimSpace(body.CompanyName)
        body.Email = strings.ToLower(strings.TrimSpace(body.Email))

        if body.Name == "" || len(body.Name) > 100 ||
                body.CompanyName == "" || len(body.CompanyName) > 150 ||
                !strings.Contains(body.Email, "@") || len(body.Email) > 255 ||
                len(body.Password) < 8 || len(body.Password) > 72 {
                c.JSON(http.StatusBadRequest, gin.H{
                        "error": "name, company name, valid email and password (8-72 bytes) are required",
                })
                return
        }

        // The platform owner account is never created through public registration.
        if body.Email == "gujjarpraveen397@gmail.com" {
                c.JSON(http.StatusConflict, gin.H{
                        "error": "this account is reserved; please sign in",
                })
                return
        }

        if jwtSecret() == nil {
                c.JSON(http.StatusInternalServerError,
                        gin.H{"error": "authentication is not configured"})
                return
        }

        hash, err := bcrypt.GenerateFromPassword(
                []byte(body.Password), bcrypt.DefaultCost,
        )
        if err != nil {
                fail(c, err)
                return
        }

        tx, err := db.BeginTx(c.Request.Context(), nil)
        if err != nil {
                fail(c, err)
                return
        }
        defer tx.Rollback()

        var userID int
        err = tx.QueryRowContext(c.Request.Context(),
                `INSERT INTO users (name, email, password_hash, platform_role)
                 VALUES ($1, $2, $3, 'user') RETURNING id`,
                body.Name, body.Email, string(hash),
        ).Scan(&userID)

        if err != nil {
                c.JSON(http.StatusConflict,
                        gin.H{"error": "email may already be registered"})
                return
        }

        // Create a unique company workspace for this registration.
        slug := strings.ToLower(body.CompanyName)
        var slugBuilder strings.Builder
        lastHyphen := false
        for _, r := range slug {
                if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
                        slugBuilder.WriteRune(r)
                        lastHyphen = false
                } else if !lastHyphen && slugBuilder.Len() > 0 {
                        slugBuilder.WriteByte('-')
                        lastHyphen = true
                }
        }
        baseSlug := strings.Trim(slugBuilder.String(), "-")
        if baseSlug == "" {
                baseSlug = "company"
        }
        if len(baseSlug) > 75 {
                baseSlug = strings.TrimRight(baseSlug[:75], "-")
        }
        companySlug := baseSlug + "-" + strconv.Itoa(userID)

        var organizationID int
        err = tx.QueryRowContext(c.Request.Context(),
                `INSERT INTO organizations (name, slug)
                 VALUES ($1, $2) RETURNING id`,
                body.CompanyName, companySlug,
        ).Scan(&organizationID)
        if err != nil {
                fail(c, err)
                return
        }

        _, err = tx.ExecContext(c.Request.Context(),
                `INSERT INTO organization_members
                 (organization_id, user_id, role, can_access_incidents)
                 VALUES ($1, $2, 'company_admin', TRUE)`,
                organizationID, userID,
        )
        if err != nil {
                fail(c, err)
                return
        }

        if err = tx.Commit(); err != nil {
                fail(c, err)
                return
        }

        issueAuthToken(c, userID, body.Name, body.Email)
}

func login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if jwtSecret() == nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "authentication is not configured"})
		return
	}

	var userID int
	var name, email, passwordHash string

	err := db.QueryRowContext(c.Request.Context(),
		`SELECT id, name, email, password_hash
		 FROM users
		 WHERE lower(email)=$1 AND is_legacy=FALSE`,
		strings.ToLower(strings.TrimSpace(body.Email)),
	).Scan(&userID, &name, &email, &passwordHash)

	if err != nil ||
		bcrypt.CompareHashAndPassword(
			[]byte(passwordHash), []byte(body.Password),
		) != nil {
		c.JSON(http.StatusUnauthorized,
			gin.H{"error": "invalid email or password"})
		return
	}

	issueAuthToken(c, userID, name, email)
}

func issueAuthToken(c *gin.Context, userID int, name, email string) {
	claims := UserClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "user",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(jwtSecret())

	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": signed,
		"user": gin.H{
			"id":    userID,
			"name":  name,
			"email": email,
		},
	})
}
