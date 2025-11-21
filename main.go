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
	dsn := os.Getenv("DATABASE_URL")

	if dsn != "" {
		log.Println("Đang kết nối PostgreSQL...")
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	} else {
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

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// --- API 1: Nhận tin SOS ---
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

	// --- API 2: Lấy danh sách ---
	r.GET("/api/sos", func(c *gin.Context) {
		var victims []Victim
		db.Where("status = ?", "PENDING").Order("created_at desc").Find(&victims)
		c.JSON(http.StatusOK, victims)
	})

	// --- API 3: Đánh dấu đã cứu (ĐÃ CHUYỂN LÊN TRÊN) ---
	r.POST("/api/sos/done", func(c *gin.Context) {
		var req struct {
			ID   uint   `json:"id"`
			Code string `json:"code"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
			return
		}

		// Mã bảo mật của bạn: DBLMM
		if req.Code != "DBLMM" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sai mã đội cứu hộ!"})
			return
		}

		// Update trạng thái
		if err := db.Model(&Victim{}).Where("id = ?", req.ID).Update("status", "SAVED").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi database"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật trạng thái thành công!"})
	})

	// --- API check server ---
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Server SOS Running OK!")
	})

	// --- KHỞI ĐỘNG SERVER (DÒNG NÀY PHẢI LUÔN NẰM CUỐI CÙNG) ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
