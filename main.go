// main.go
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/cors"
)

var chanFile chan string

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	uuidString := uuid.New().String()
	fileName := fmt.Sprintf("video/%s.mp4", uuidString)
	file, err := os.Create(fileName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	chanFile <- uuidString

	w.WriteHeader(http.StatusOK)
}

// Hàm chuyển đổi video sang chất lượng thấp hơn sử dụng ffmpeg
func convertVideoToLowQuality(inputFile string, outputFile string) error {
	// Gọi ffmpeg để chuyển đổi video sang chất lượng thấp hơn
	cmd := exec.Command(
		"ffmpeg",
		"-i", inputFile,
		"-vf", "scale=640:360",
		"-vcodec", "libx264",
		outputFile,
	)

	// Gửi output của ffmpeg vào terminal để debug nếu cần
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Thực thi lệnh
	return cmd.Run()
}

func main() {
	chanFile = make(chan string)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Println("start chanel")
		for uuidString := range chanFile {
			fileName := fmt.Sprintf("video/%s.mp4", uuidString)
			lowFile := fmt.Sprintf("low/%s.mp4", uuidString)
			err := convertVideoToLowQuality(fileName, lowFile)
			if err != nil {
				log.Println(err)
			}
		}
		wg.Done()
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/upload", uploadHandler)

	// Sử dụng thư viện CORS
	handler := cors.Default().Handler(mux)
	http.ListenAndServe(":8080", handler)

	wg.Wait()
}
