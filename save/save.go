package save

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Cho phép tất cả các nguồn (cẩn thận với điều này trong môi trường sản xuất)
	},
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	// Đọc thông điệp từ WebSocket và gửi đến các tiến trình FFmpeg
	dir := "./hls/hls.tmp" // Tạo HLS stream trong từng thư mục riêng
	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-c:v", "h264_nvenc",
		"-preset", "slow",
		"-profile:v", "high",
		"-rc", "constqp",
		"-crf", "18", // Điều chỉnh giá trị CRF (giá trị nhỏ hơn = chất lượng cao hơn)
		"-b:v", "3000k", // Tăng bitrate lên 3000 kbps
		"-r", "60", // Giữ FPS ở mức 60 khung hình mỗi giây
		"-g", "60", // Khoảng cách giữa các keyframe (mỗi giây 1 keyframe)
		"-sc_threshold", "0",
		"-pix_fmt", "yuv420p",
		"-hls_time", "1", // Chia đoạn HLS thành 2 giây
		"-hls_list_size", "0",
		"-f", "hls",
		dir, // Đầu ra của HLS cho từng thư mục
	)
	ffmpegStdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal("Error creating FFmpeg stdin pipe:", err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		log.Fatal("Failed to start FFmpeg:", err)
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		n, err := ffmpegStdin.Write(message)
		log.Println(n)
		if err != nil {
			log.Printf("Failed to write to FFmpeg stdin %v", err)
		}
	}

	// Đóng các pipe và chờ các tiến trình kết thúc
	ffmpegStdin.Close()
	cmd.Wait()
}

// Middleware để xử lý CORS
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Cho phép tất cả các nguồn
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return // Trả về cho các yêu cầu OPTIONS
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)

	// Bọc serve HLS với middleware CORS
	http.Handle("/hls/", corsMiddleware(http.StripPrefix("/hls/", http.FileServer(http.Dir("./hls")))))

	// Bọc server chính với middleware CORS
	http.Handle("/", corsMiddleware(http.DefaultServeMux))

	fmt.Println("Server started at :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
