package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"gopkg.in/ini.v1"
)

var (
	fileOffsets = make(map[string]int64)
	upgrader    = websocket.Upgrader{}
	clients     = make(map[*websocket.Conn]bool)
	mutex       sync.Mutex
	username    string
	password    string
	host        string
	port        string
	logDirs     []string
)

func main() {
	cfg, err := ini.Load("monitor_config.ini")
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	username = cfg.Section("auth").Key("username").String()
	password = cfg.Section("auth").Key("password").String()
	host = cfg.Section("server").Key("host").String()
	port = cfg.Section("server").Key("port").String()

	// Read log directories from the configuration
	logDirsStr := cfg.Section("logs").Key("directories").String()
	logDirs = strings.Split(logDirsStr, ",")

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Handler:      nil,
		ReadTimeout:  10 * time.Second,  // Adjust as necessary
		WriteTimeout: 10 * time.Second,  // Adjust as necessary
		IdleTimeout:  120 * time.Second, // Adjust as necessary
	}

	log.Printf("Starting server on %s", fmt.Sprintf("%s:%s", host, port))

	// Serve static files from the static directory
	http.Handle("/", basicAuth(http.FileServer(http.Dir("./static"))))
	// Handle WebSocket connections
	http.Handle("/ws", basicAuth(http.HandlerFunc(handleWebSocket)))

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Initialize file offsets and add directories to watcher
	for _, logDir := range logDirs {
		if err := initializeFileOffsets(logDir); err != nil {
			log.Fatalf("Failed to initialize file offsets for %s: %v", logDir, err)
		}
		if err := watcher.Add(logDir); err != nil {
			log.Fatalf("Failed to add directory %s to watcher: %v", logDir, err)
		}
	}

	go watchFiles(watcher)

	done := make(chan struct{})
	<-done
}

func basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || pair[0] != username || pair[1] != password {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			mutex.Lock()
			delete(clients, conn)
			mutex.Unlock()
			break
		}
	}
}

func broadcastLog(message string) {
	mutex.Lock()
	defer mutex.Unlock()
	for client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			log.Printf("Failed to send message to client: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func watchFiles(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if strings.HasSuffix(event.Name, ".log") && (event.Op&fsnotify.Write == fsnotify.Write) {
				lines, path := fileModify(event.Name)
				if len(lines) > 0 {
					message := fmt.Sprintf("File: %s\n%s", path, strings.Join(lines, "\n"))
					broadcastLog(message)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func initializeFileOffsets(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking directory: %w", err)
		}
		if strings.HasSuffix(d.Name(), ".log") {
			offset, err := getFileOffset(path)
			if err != nil {
				return fmt.Errorf("error initializing file %s: %w", path, err)
			}
			fileOffsets[path] = offset
			log.Printf("File: %s", path)
		}
		return nil
	})
}

func getFileOffset(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	return offset, nil
}

func fileModify(file string) ([]string, string) {
	f, err := os.Open(file)
	if err != nil {
		log.Printf("Error opening file %s: %v", file, err)
		return nil, file
	}
	defer f.Close()

	offset := fileOffsets[file]
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		log.Printf("Error seeking file %s: %v", file, err)
		return nil, file
	}

	var newLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		newLines = append(newLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading file %s: %v", file, err)
		return nil, file
	}

	newOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		log.Printf("Error getting new offset for file %s: %v", file, err)
		return nil, file
	}
	fileOffsets[file] = newOffset

	return newLines, file
}
