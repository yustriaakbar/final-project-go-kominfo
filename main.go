package main

import (
	"fmt"
	"net/http"
	"time"

	// "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"golang.org/x/crypto/bcrypt"
)

var db *gorm.DB

// var err error

// User model
type User struct {
	ID           uint          `gorm:"primary_key" json:"id"`
	Username     string        `gorm:"unique;not null" json:"username"`
	Email        string        `gorm:"unique;not null" json:"email"`
	Password     string        `gorm:"not null" json:"password"`
	Age          uint          `gorm:"not null" json:"age"`
	CreatedAt    time.Time     `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time     `gorm:"column:updated_at" json:"updated_at"`
	Photos       []Photo       `json:"photos"`
	Comments     []Comment     `json:"comments"`
	SocialMedias []SocialMedia `json:"social_medias"`
}

// Photo model
type Photo struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	Title     string    `gorm:"not null" json:"title"`
	Caption   string    `gorm:"not null" json:"caption"`
	PhotoURL  string    `gorm:"not null" json:"photo_url"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// Comment model
type Comment struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	PhotoID   uint      `gorm:"not null" json:"photo_id"`
	Message   string    `gorm:"not null" json:"message"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// SocialMedia model
type SocialMedia struct {
	ID             uint      `gorm:"primary_key" json:"id"`
	Name           string    `gorm:"not null" json:"name"`
	SocialMediaURL string    `gorm:"not null" json:"social_media_url"`
	UserID         uint      `gorm:"not null" json:"user_id"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// JWT claims struct
type Claims struct {
	UserID uint `json:"user_id"`
	jwt.StandardClaims
}

func main() {
	// Initialize Gin
	r := gin.Default()

	// Connect to the database
	var err error
	db, err = gorm.Open("mysql", "root:@tcp(127.0.0.1:3306)/mygram?parseTime=true")

	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()

	// Migrate the schema
	// db.AutoMigrate(&User{}, &Photo{}, &Comment{}, &SocialMedia{})

	// Routes
	r.POST("/users/register", registerUser)
	r.POST("/users/login", loginUser)

	// Authorized routes
	authorized := r.Group("/")
	authorized.Use(authMiddleware)
	{
		authorized.PUT("/users", updateUser)
		authorized.DELETE("/users", deleteUser)

		authorized.POST("/photos", createPhoto)
		authorized.GET("/photos", getPhotos)
		authorized.PUT("/photos/:photoId", updatePhoto)
		authorized.DELETE("/photos/:photoId", deletePhoto)

		authorized.POST("/comments", createComment)
		authorized.GET("/comments", getComments)
		authorized.PUT("/comments/:commentId", updateComment)
		authorized.DELETE("/comments/:commentId", deleteComment)

		authorized.POST("/socialmedias", createSocialMedia)
		authorized.GET("/socialmedias", getSocialMedias)
		authorized.PUT("/socialmedias/:socialMediaId", updateSocialMedia)
		authorized.DELETE("/socialmedias/:socialMediaId", deleteSocialMedia)
	}

	// Run the server
	r.Run(":9090")
}

// Middleware to authenticate requests
func authMiddleware(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}

	c.Set("userId", claims.UserID)
	c.Next()
}

// User registration handler
func registerUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash password before saving
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	user.Password = string(hashedPassword)

	// Save user to database
	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// User login handler
func loginUser(c *gin.Context) {
	var user User
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&loginData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Temukan pengguna berdasarkan email
	if err := db.Where("email = ?", loginData.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password #1"})
		return
	}

	// Bandingkan hash password di database dengan password yang diterima dari pengguna
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginData.Password)); err != nil {
		fmt.Println("Hash comparison failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password #2"})
		return
	}

	// Buat token JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: user.ID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(), // 1 day
		},
	})

	// Generate encoded token and send it as response
	tokenString, err := token.SignedString([]byte("secret"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

// Update user handler
func updateUser(c *gin.Context) {
	userId, _ := c.Get("userId")
	var user User
	if err := db.Where("id = ?", userId).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var updateUser User
	if err := c.ShouldBindJSON(&updateUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.Model(&user).Updates(&updateUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Delete user handler
func deleteUser(c *gin.Context) {
	userId, _ := c.Get("userId")
	var user User
	if err := db.Where("id = ?", userId).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Your account has been successfully deleted"})
}

// Create photo handler
func createPhoto(c *gin.Context) {
	userId, _ := c.Get("userId")
	var photo Photo
	if err := c.ShouldBindJSON(&photo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	photo.UserID = userId.(uint)

	if err := db.Create(&photo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create photo"})
		return
	}

	c.JSON(http.StatusCreated, photo)
}

// Get all photos handler
func getPhotos(c *gin.Context) {
	var photos []Photo
	if err := db.Find(&photos).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch photos"})
		return
	}

	c.JSON(http.StatusOK, photos)
}

// Update photo handler
func updatePhoto(c *gin.Context) {
	userId, _ := c.Get("userId")
	photoId := c.Param("photoId")
	var photo Photo
	if err := db.Where("id = ?", photoId).First(&photo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
		return
	}

	if photo.UserID != userId.(uint) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var updatePhoto Photo
	if err := c.ShouldBindJSON(&updatePhoto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.Model(&photo).Updates(&updatePhoto).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update photo"})
		return
	}

	c.JSON(http.StatusOK, photo)
}

// Delete photo handler
func deletePhoto(c *gin.Context) {
	userId, _ := c.Get("userId")
	photoId := c.Param("photoId")
	var photo Photo
	if err := db.Where("id = ?", photoId).First(&photo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
		return
	}

	if photo.UserID != userId.(uint) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := db.Delete(&photo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete photo"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Your photo has been successfully deleted"})
}

// Create comment handler
func createComment(c *gin.Context) {
	userId, _ := c.Get("userId")
	var comment Comment
	if err := c.ShouldBindJSON(&comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	comment.UserID = userId.(uint)

	if err := db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	c.JSON(http.StatusCreated, comment)
}

// Get all comments handler
func getComments(c *gin.Context) {
	var comments []Comment
	if err := db.Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}

	c.JSON(http.StatusOK, comments)
}

// Update comment handler
func updateComment(c *gin.Context) {
	userId, _ := c.Get("userId")
	commentId := c.Param("commentId")
	var comment Comment
	if err := db.Where("id = ?", commentId).First(&comment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"})
		return
	}

	if comment.UserID != userId.(uint) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var updateComment Comment
	if err := c.ShouldBindJSON(&updateComment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.Model(&comment).Updates(&updateComment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update comment"})
		return
	}

	c.JSON(http.StatusOK, comment)
}

// Delete comment handler
func deleteComment(c *gin.Context) {
	userId, _ := c.Get("userId")
	commentId := c.Param("commentId")
	var comment Comment
	if err := db.Where("id = ?", commentId).First(&comment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"})
		return
	}

	if comment.UserID != userId.(uint) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := db.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Your comment has been successfully deleted"})
}

// Create social media handler
func createSocialMedia(c *gin.Context) {
	userId, _ := c.Get("userId")
	var socialMedia SocialMedia
	if err := c.ShouldBindJSON(&socialMedia); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	socialMedia.UserID = userId.(uint)

	if err := db.Create(&socialMedia).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create social media"})
		return
	}

	c.JSON(http.StatusCreated, socialMedia)
}

// Get all social medias handler
func getSocialMedias(c *gin.Context) {
	var socialMedias []SocialMedia
	if err := db.Find(&socialMedias).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch social medias"})
		return
	}

	c.JSON(http.StatusOK, socialMedias)
}

// Update social media handler
func updateSocialMedia(c *gin.Context) {
	userId, _ := c.Get("userId")
	socialMediaId := c.Param("socialMediaId")
	var socialMedia SocialMedia
	if err := db.Where("id = ?", socialMediaId).First(&socialMedia).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Social media not found"})
		return
	}

	if socialMedia.UserID != userId.(uint) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var updateSocialMedia SocialMedia
	if err := c.ShouldBindJSON(&updateSocialMedia); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.Model(&socialMedia).Updates(&updateSocialMedia).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update social media"})
		return
	}

	c.JSON(http.StatusOK, socialMedia)
}

// Delete social media handler
func deleteSocialMedia(c *gin.Context) {
	userId, _ := c.Get("userId")
	socialMediaId := c.Param("socialMediaId")
	var socialMedia SocialMedia
	if err := db.Where("id = ?", socialMediaId).First(&socialMedia).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Social media not found"})
		return
	}

	if socialMedia.UserID != userId.(uint) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := db.Delete(&socialMedia).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete social media"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Your social media has been successfully deleted"})
}
