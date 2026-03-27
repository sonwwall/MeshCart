package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	logx "meshcart/app/log"
	mqx "meshcart/app/mq"
	"meshcart/services/payment-service/biz/repository"
	"meshcart/services/payment-service/config"
	"meshcart/services/payment-service/dal/db"
	dalmodel "meshcart/services/payment-service/dal/model"
)

type paymentSucceededPayload struct {
	PaymentID      int64  `json:"payment_id"`
	OrderID        int64  `json:"order_id"`
	UserID         int64  `json:"user_id"`
	Amount         int64  `json:"amount"`
	PaymentMethod  string `json:"payment_method"`
	PaymentTradeNo string `json:"payment_trade_no"`
	SucceededAt    int64  `json:"succeeded_at"`
}

func Run() {
	initLogger()
	defer logx.Sync()

	cfg, err := config.Load()
	if err != nil {
		logx.L(nil).Fatal("load config failed", zap.Error(err))
	}
	mysqlDB, err := db.NewMySQL(cfg.MySQL.DSN())
	if err != nil {
		logx.L(nil).Fatal("init mysql failed", zap.Error(err))
	}
	sqlDB, err := mysqlDB.DB()
	if err != nil {
		logx.L(nil).Fatal("get mysql sql db failed", zap.Error(err))
	}
	defer sqlDB.Close()

	repo := repository.NewMySQLPaymentRepository(mysqlDB, time.Duration(cfg.Timeout.DBQueryMS)*time.Millisecond)
	node, err := snowflake.NewNode(cfg.Snowflake.Node + 100)
	if err != nil {
		logx.L(nil).Fatal("init snowflake node failed", zap.Error(err))
	}

	groupID := cfg.MQ.PaymentSucceededConsumerGroup
	reader := mqx.NewKafkaReader(cfg.MQ.Brokers, groupID, cfg.MQ.PaymentSucceededTopic)
	if reader == nil {
		logx.L(nil).Fatal("init kafka reader failed")
	}
	defer reader.Close()

	logx.L(nil).Info("payment consumer starting",
		zap.String("group_id", groupID),
		zap.String("topic", cfg.MQ.PaymentSucceededTopic),
		zap.Strings("brokers", cfg.MQ.Brokers),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logx.L(nil).Error("fetch kafka message failed", zap.Error(err))
			continue
		}
		if err := handleMessage(ctx, repo, node, groupID, msg); err != nil {
			logx.L(nil).Error("handle kafka message failed", zap.Error(err))
			continue
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			logx.L(nil).Error("commit kafka message failed", zap.Error(err))
		}
	}
}

func handleMessage(ctx context.Context, repo repository.PaymentRepository, node *snowflake.Node, groupID string, msg kafka.Message) error {
	var envelope mqx.Envelope
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		return err
	}
	if err := envelope.Validate(); err != nil {
		return err
	}

	record, err := repo.GetConsumeRecord(ctx, groupID, envelope.ID)
	if err == nil {
		if record.Status == repository.ConsumeStatusSucceeded {
			logx.L(nil).Info("payment consumer skipped duplicated succeeded event",
				zap.String("group_id", groupID),
				zap.String("event_id", envelope.ID),
				zap.String("event_name", envelope.EventName),
			)
			return nil
		}
		return nil
	}
	if err != repository.ErrActionRecordNotFound {
		return err
	}

	record = &dalmodel.PaymentConsumeRecord{
		ID:            node.Generate().Int64(),
		ConsumerGroup: groupID,
		EventID:       envelope.ID,
		EventName:     envelope.EventName,
		Status:        repository.ConsumeStatusPending,
	}
	if err := repo.CreateConsumeRecord(ctx, record); err != nil {
		if err == repository.ErrActionRecordExists {
			return nil
		}
		return err
	}

	switch envelope.EventName {
	case "payment.succeeded":
		var payload paymentSucceededPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			_ = repo.MarkConsumeRecordFailed(ctx, record.ID, err.Error())
			return err
		}
		logx.L(nil).Info("payment consumer processed payment.succeeded",
			zap.String("event_id", envelope.ID),
			zap.Int64("payment_id", payload.PaymentID),
			zap.Int64("order_id", payload.OrderID),
			zap.Int64("user_id", payload.UserID),
			zap.Int64("amount", payload.Amount),
			zap.String("payment_method", payload.PaymentMethod),
		)
	default:
		logx.L(nil).Warn("payment consumer ignored unsupported event",
			zap.String("event_id", envelope.ID),
			zap.String("event_name", envelope.EventName),
		)
	}

	return repo.MarkConsumeRecordSucceeded(ctx, record.ID)
}

func initLogger() {
	if err := logx.Init(logx.Config{
		Service: "payment-consumer",
		Env:     getEnv("APP_ENV", "dev"),
		Level:   getEnv("LOG_LEVEL", "info"),
		LogDir:  getEnv("LOG_DIR", "logs"),
	}); err != nil {
		panic(err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
