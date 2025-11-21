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
	Status    string    `json:"status"` // PENDING, SAVED
	Type      string    `json:"type"`   // "SOS" (Khẩn cấp) hoặc "FOOD" (Lương thực)
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
	// Tự động thêm cột Type nếu chưa có
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

	// --- API 1: Nhận tin ---
	r.POST("/api/sos", func(c *gin.Context) {
		var input Victim
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// LOGIC CHỐNG TRÙNG LẶP (QUAN TRỌNG)
		// Nếu SĐT này đang có tin PENDING, ta cập nhật lại tọa độ/thời gian chứ không tạo mới
		var existing Victim
		result := db.Where("phone = ? AND status = ?", input.Phone, "PENDING").First(&existing)

		if result.Error == nil {
			// Đã tồn tại -> Cập nhật
			existing.Lat = input.Lat
			existing.Long = input.Long
			existing.Name = input.Name
			existing.Type = input.Type
			existing.CreatedAt = time.Now() // Làm mới thời gian
			db.Save(&existing)
			c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật vị trí mới", "data": existing})
		} else {
			// Chưa tồn tại -> Tạo mới
			input.Status = "PENDING"
			input.CreatedAt = time.Now()
			db.Create(&input)
			c.JSON(http.StatusOK, gin.H{"message": "Đã nhận tín hiệu", "data": input})
		}
	})

	// API MỚI: Hủy tin SOS (Dùng khi người dân bấm "Gửi lại")
	r.POST("/api/sos/cancel", func(c *gin.Context) {
		var req struct {
			ID    uint   `json:"id"`
			Phone string `json:"phone"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu lỗi"})
			return
		}

		// Xóa cứng khỏi database để dọn rác
		db.Where("id = ? AND phone = ?", req.ID, req.Phone).Delete(&Victim{})
		c.JSON(http.StatusOK, gin.H{"message": "Đã xóa tin cũ"})
	})

	// --- API 2: Lấy danh sách (Lấy hết PENDING) ---
	r.GET("/api/sos", func(c *gin.Context) {
		var victims []Victim
		// Lấy tất cả pending, frontend sẽ tự filter theo Tab
		db.Where("status = ?", "PENDING").Order("created_at desc").Find(&victims)
		c.JSON(http.StatusOK, victims)
	})

	// --- API 3: Đánh dấu đã xong ---
	r.POST("/api/sos/done", func(c *gin.Context) {
		var req struct {
			ID   uint   `json:"id"`
			Code string `json:"code"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
			return
		}

		if req.Code != "DBLMM" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sai mã đội cứu hộ!"})
			return
		}

		if err := db.Model(&Victim{}).Where("id = ?", req.ID).Update("status", "SAVED").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi database"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật trạng thái thành công!"})
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
