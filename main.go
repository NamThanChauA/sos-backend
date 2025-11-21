package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Victim struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Phone     string    `json:"phone"`
	Name      string    `json:"name"`
	Lat       float64   `json:"lat"`
	Long      float64   `json:"long"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

var db *gorm.DB

func initDB() {
	var err error
	// Lấy chuỗi kết nối từ biến môi trường (Render sẽ cung cấp cái này)
	dsn := os.Getenv("DATABASE_URL")

	if dsn != "" {
		// Nếu có biến môi trường -> Dùng Postgres (Trên Render)
		log.Println("Đang kết nối PostgreSQL...")
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	} else {
		// Nếu không có -> Dùng SQLite (Trên máy tính cá nhân)
		log.Println("Đang dùng SQLite local...")
		db, err = gorm.Open(sqlite.Open("sos.db"), &gorm.Config{})
	}

	if err != nil {
		log.Fatal("Lỗi kết nối Database:", err)
	}
	db.AutoMigrate(&Victim{})
}

func main() {
	initDB()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS: Cho phép tất cả các web gọi vào
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.POST("/api/sos", func(c *gin.Context) {
		var input Victim
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		input.Status = "PENDING"
		input.CreatedAt = time.Now()

		if result := db.Create(&input); result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi lưu database"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Đã nhận tín hiệu", "data": input})
	})

	r.GET("/api/sos", func(c *gin.Context) {
		var victims []Victim
		db.Order("created_at desc").Find(&victims)
		c.JSON(http.StatusOK, victims)
	})

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Server SOS Running OK!")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
