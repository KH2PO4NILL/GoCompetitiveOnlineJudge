package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, "isolated auth fixture setup failed")
		os.Exit(1)
	}
}

func prepare() error {
	if os.Getenv("GCOJ_AUTH_E2E") != "1" {
		return fmt.Errorf("isolated test mode required")
	}
	directory := os.Getenv("GCOJ_AUTH_E2E_RUNTIME")
	password := os.Getenv("GCOJ_AUTH_E2E_PASSWORD")
	if directory == "" || len(password) < 16 {
		return fmt.Errorf("missing private runtime or random test credential")
	}
	database, err := sql.Open("mysql", os.Getenv("GCOJ_AUTH_E2E_DSN"))
	if err != nil {
		return err
	}
	defer database.Close()
	var databaseName string
	if err := database.QueryRow("SELECT DATABASE()").Scan(&databaseName); err != nil || databaseName != "auth_e2e" {
		return fmt.Errorf("fixture database must be auth_e2e")
	}
	for _, migration := range []string{"001_create_user.sql", "002_create_school.sql", "003_create_user_sessions.sql"} {
		contents, err := os.ReadFile(filepath.Join("services/grpc/auth/migrations", migration))
		if err != nil {
			return err
		}
		if _, err := database.Exec(string(contents)); err != nil {
			return err
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	for _, account := range []struct {
		username string
		role     int
	}{{"e2e-user", 1}, {"e2e-admin", 2}} {
		if _, err := database.Exec("INSERT INTO users (username, email, password_hash, role, status) VALUES (?, ?, ?, ?, 1)", account.username, account.username+"@example.invalid", string(hash), account.role); err != nil {
			return err
		}
	}
	authPort, err := availablePort()
	if err != nil {
		return err
	}
	appPort, err := availablePort()
	if err != nil {
		return err
	}
	adminPort, err := availablePort()
	if err != nil {
		return err
	}
	authAddress := fmt.Sprintf("127.0.0.1:%d", authPort)
	auth := map[string]any{
		"Name": "auth.web-e2e", "ListenOn": authAddress, "Mode": "test",
		"Log":          map[string]any{"Mode": "console", "Level": "error"},
		"Middlewares":  map[string]any{"Stat": false},
		"Mysql":        map[string]any{"DSN": os.Getenv("GCOJ_AUTH_E2E_DSN")},
		"RedisDB":      map[string]any{"Addr": os.Getenv("GCOJ_AUTH_E2E_REDIS"), "Password": "", "DB": 0},
		"Token":        map[string]any{"Secret": password + password, "Issuer": "auth.web-e2e", "Audience": "gcoj-web-e2e", "AccessExpireSeconds": 3, "RefreshExpireSeconds": 3600, "LeewaySeconds": 0},
		"Mail":         map[string]any{"Host": "localhost", "Port": 465, "Username": "unused", "Password": password, "From": "test@example.invalid", "Security": "ssl"},
		"Session":      map[string]any{"CleanupIntervalSeconds": 60, "RevokedRetentionSeconds": 60},
		"InternalAuth": map[string]any{"Callers": []any{map[string]any{"Name": "web-e2e.rpc", "Token": password, "Methods": []string{"/auth.AuthService/ValidateToken"}}}},
	}
	if err := writeJSON(directory, "auth.json", auth); err != nil {
		return err
	}
	for _, gateway := range []struct {
		name string
		port int
	}{{"app", appPort}, {"admin", adminPort}} {
		config := map[string]any{
			"Name": gateway.name + ".web-e2e", "Host": "127.0.0.1", "Port": gateway.port, "Mode": "test", "Timeout": 10000,
			"Log":     map[string]any{"Mode": "console", "Level": "error"},
			"AuthRPC": map[string]any{"Endpoints": []string{authAddress}, "Timeout": 5000},
		}
		for _, service := range []string{"ProblemRPC", "ContestRPC", "SubmitRPC"} {
			config[service] = map[string]any{"Endpoints": []string{"127.0.0.1:1"}, "NonBlock": true}
		}
		if err := writeJSON(directory, gateway.name+".json", config); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(directory, "ports"), []byte(fmt.Sprintf("%d %d %d\n", authPort, appPort, adminPort)), 0600)
}

func availablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	return port, listener.Close()
}

func writeJSON(directory, name string, value any) error {
	contents, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, name), contents, 0600)
}
