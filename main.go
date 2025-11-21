package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite" // <--- Đã đổi sang thư viện thuần Go
	"gorm.io/gorm"
)

// Cấu trúc dữ liệu nạn nhân
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

// Hàm khởi tạo Database
func initDB() {
	var err error
	// Sử dụng thư viện glebarez/sqlite không cần CGO
	db, err = gorm.Open(sqlite.Open("sos.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Không thể tạo Database:", err)
	}
	db.AutoMigrate(&Victim{})
}

func main() {
	initDB()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Cấu hình CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API Nhận tin
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

	// API Xem danh sách
	r.GET("/api/sos", func(c *gin.Context) {
		var victims []Victim
		db.Order("created_at desc").Find(&victims)
		c.JSON(http.StatusOK, victims)
	})

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Server SOS đang chạy ngon lành!")
	})

	log.Println("Server đang chạy tại http://localhost:8080")
	r.Run(":8080")
}
