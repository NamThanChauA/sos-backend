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
	Phone     string    `json:"phone" gorm:"index"` // Thêm index để tìm cho nhanh
	Name      string    `json:"name"`
	Lat       float64   `json:"lat"`
	Long      float64   `json:"long"`
	Status    string    `json:"status"` // PENDING (Chờ), SAVED (Đã cứu), CANCELLED (Đã hủy)
	Type      string    `json:"type"`   // SOS, SUPPLY
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

	// --- API 1: Gửi / Cập nhật tin SOS (LOGIC QUAN TRỌNG NHẤT) ---
	r.POST("/api/sos", func(c *gin.Context) {
		var input Victim
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Tìm bản ghi cũ theo Số điện thoại (Bất kể trạng thái là gì)
		var existing Victim
		result := db.Where("phone = ?", input.Phone).First(&existing)

		if result.Error == nil {
			// === TRƯỜNG HỢP ĐÃ CÓ TRONG DB ===
			// Ta "Hồi sinh" bản ghi này lại
			existing.Lat = input.Lat
			existing.Long = input.Long
			existing.Name = input.Name
			existing.Type = input.Type
			existing.Status = "PENDING"     // Đưa về trạng thái chờ cứu
			existing.CreatedAt = time.Now() // Cập nhật thời gian mới nhất để lên đầu list

			db.Save(&existing)
			c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật thông tin cứu hộ mới", "data": existing})
		} else {
			// === TRƯỜNG HỢP SĐT MỚI TINH ===
			input.Status = "PENDING"
			input.CreatedAt = time.Now()

			if err := db.Create(&input).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi database"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Đã nhận tín hiệu mới", "data": input})
		}
	})

	// --- API 2: Hủy tin SOS (Soft Delete - Chuyển trạng thái CANCELLED) ---
	r.POST("/api/sos/cancel", func(c *gin.Context) {
		var req struct {
			Phone string `json:"phone"`
		}
		// Frontend chỉ cần gửi { "phone": "09123..." } là đủ
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu lỗi"})
			return
		}

		// Tìm và chuyển status thành CANCELLED
		// Chỉ update nếu đang là PENDING (để tránh sửa nhầm các ca đã SAVED)
		result := db.Model(&Victim{}).Where("phone = ? AND status = ?", req.Phone, "PENDING").Update("status", "CANCELLED")

		if result.RowsAffected == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "Không tìm thấy tin nào cần hủy hoặc đã được xử lý rồi"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Đã hủy yêu cầu cứu hộ"})
	})

	// --- API 3: Lấy danh sách (Chỉ lấy PENDING) ---
	r.GET("/api/sos", func(c *gin.Context) {
		var victims []Victim
		// Tuyệt đối chỉ lấy status = PENDING.
		// SAVED và CANCELLED sẽ bị ẩn khỏi danh sách này.
		db.Where("status = ?", "PENDING").Order("created_at desc").Find(&victims)
		c.JSON(http.StatusOK, victims)
	})

	// --- API 4: Đánh dấu đã cứu (Cần mã DBLMM) ---
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

		// Update thành SAVED
		if err := db.Model(&Victim{}).Where("id = ?", req.ID).Update("status", "SAVED").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi database"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Đã đánh dấu cứu thành công!"})
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
