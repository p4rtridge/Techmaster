package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// OCRResult định nghĩa cấu trúc kết quả từ PaddleOCR
type OCRResult struct {
	Coords     [][2]float64 `json:"coords"`
	Text       string       `json:"text"`
	Confidence float64      `json:"confidence"`
}

const MAX_ALLOWED_DIMENSION = 800

func main() {
	// Tạo thư mục tạm thời để lưu ảnh
	os.MkdirAll("./temp", os.ModePerm)

	// Sử dụng middleware CORS
	http.HandleFunc("/ocr", corsMiddleware(handleOCR))

	port := 8080
	fmt.Printf("Server is running on port %d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// Middleware để xử lý CORS
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Thiết lập CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// Xử lý preflight request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Gọi handler tiếp theo
		next(w, r)
	}
}

func handleOCR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(20 << 20)

	// Lấy file từ request
	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Error retrieving the file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

	// Sử dụng giá trị mặc định là kích thước tối đa
	maxWidth := MAX_ALLOWED_DIMENSION
	maxHeight := MAX_ALLOWED_DIMENSION

	// Xử lý tham số max_width - đảm bảo không vượt quá giới hạn
	if widthStr := r.FormValue("max_width"); widthStr != "" {
		if width, err := strconv.Atoi(widthStr); err == nil && width > 0 {
			maxWidth = width
		}
	}

	// Xử lý tham số max_height - đảm bảo không vượt quá giới hạn
	if heightStr := r.FormValue("max_height"); heightStr != "" {
		if height, err := strconv.Atoi(heightStr); err == nil && height > 0 {
			maxHeight = height
		}
	}

	// Tạo tên file tạm thời dựa trên timestamp
	tempFileName := fmt.Sprintf("temp/%d_%s", time.Now().Unix(), handler.Filename)
	tempFilePath, _ := filepath.Abs(tempFileName)

	// Tạo file tạm thời
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		http.Error(w, "Error creating temporary file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()
	defer os.Remove(tempFilePath) // Xóa file sau khi xử lý xong

	// Sao chép nội dung file upload vào file tạm thời
	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Error copying file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Đóng file trước khi xử lý
	tempFile.Close()

	// Gọi PaddleOCR script để xử lý ảnh với kích thước hợp lệ
	result, err := processPaddleOCR(tempFilePath, maxWidth, maxHeight)
	if err != nil {
		http.Error(w, "Error processing image with PaddleOCR: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Trả về kết quả dưới dạng JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func processPaddleOCR(imagePath string, maxWidth, maxHeight int) ([]OCRResult, error) {
	if maxWidth > MAX_ALLOWED_DIMENSION {
		maxWidth = MAX_ALLOWED_DIMENSION
	}

	if maxHeight > MAX_ALLOWED_DIMENSION {
		maxHeight = MAX_ALLOWED_DIMENSION
	}

	// Đường dẫn tới script OCR
	scriptPath := "ocr.py"

	// Gọi script Python với các tham số: đường dẫn ảnh, chiều rộng tối đa, chiều cao tối đa
	cmd := exec.Command("python", scriptPath, imagePath, fmt.Sprintf("%d", maxWidth), fmt.Sprintf("%d", maxHeight))

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error executing PaddleOCR script: %v - %s", err, stderr.String())
	}

	// Parse kết quả JSON từ PaddleOCR
	var results []OCRResult
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		return nil, fmt.Errorf("error parsing OCR results: %v", err)
	}

	return results, nil
}
