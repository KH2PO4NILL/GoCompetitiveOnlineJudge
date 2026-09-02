package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config config.Config

	SubmitModel model.SubmitModel
	Now         func() time.Time

	db    *gorm.DB
	sqlDB *sql.DB
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if c.Mysql.DSN == "" {
		return nil, errors.New("mysql DSN is required")
	}
	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(mysql.Open(c.Mysql.DSN), &gorm.Config{Logger: gormLogger})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql database: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	clock := func() time.Time { return time.Now().UTC() }
	return &ServiceContext{
		Config: c, SubmitModel: model.NewSubmitModel(db, model.WithClock(clock)), Now: clock,
		db: db, sqlDB: sqlDB,
	}, nil
}

func (s *ServiceContext) Close() error {
	if s.sqlDB == nil {
		return nil
	}
	return s.sqlDB.Close()
}
