// Package testsupport creates disposable databases; it never migrates an existing database.
package testsupport

import (
	"database/sql"
	"fmt"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func Database(t testing.TB) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("AUTH_MODEL_TEST_DSN")
	if dsn == "" {
		t.Skip("set AUTH_MODEL_TEST_DSN for disposable Auth MySQL tests")
	}
	c, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid Auth test DSN")
	}
	c.DBName = ""
	c.ParseTime = true
	c.Loc = time.UTC
	admin, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		t.Fatal("open test admin")
	}
	name := fmt.Sprintf("auth_model_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		admin.Close()
		t.Fatal("create disposable Auth database", err)
	}
	c.DBName = name
	conn, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		t.Fatal("open test database")
	}
	t.Cleanup(func() { conn.Close(); admin.Exec("DROP DATABASE IF EXISTS `" + name + "`"); admin.Close() })
	_, file, _, _ := runtime.Caller(0)
	files, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var lines []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				lines = append(lines, line)
			}
		}
		for _, stmt := range strings.Split(strings.Join(lines, "\n"), ";") {
			if strings.TrimSpace(stmt) != "" {
				if _, err := conn.Exec(stmt); err != nil {
					t.Fatalf("migration %s: %v", filepath.Base(file), err)
				}
			}
		}
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("open GORM")
	}
	return db
}
