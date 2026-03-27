package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	dalmodel "meshcart/services/payment-service/dal/model"
)

func TestMySQLPaymentRepository_ConsumeRecordLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:payment-consume-records?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dalmodel.PaymentConsumeRecord{}); err != nil {
		t.Fatalf("migrate consume record schema: %v", err)
	}

	repo := NewMySQLPaymentRepository(db, time.Second)
	ctx := context.Background()

	record := &dalmodel.PaymentConsumeRecord{
		ID:            1,
		ConsumerGroup: "g1",
		EventID:       "evt-1",
		EventName:     "payment.succeeded",
		Status:        ConsumeStatusPending,
	}
	if err := repo.CreateConsumeRecord(ctx, record); err != nil {
		t.Fatalf("create consume record: %v", err)
	}

	got, err := repo.GetConsumeRecord(ctx, "g1", "evt-1")
	if err != nil {
		t.Fatalf("get consume record: %v", err)
	}
	if got.Status != ConsumeStatusPending {
		t.Fatalf("unexpected pending record: %+v", got)
	}

	if err := repo.MarkConsumeRecordSucceeded(ctx, 1); err != nil {
		t.Fatalf("mark consume record succeeded: %v", err)
	}
	got, err = repo.GetConsumeRecord(ctx, "g1", "evt-1")
	if err != nil {
		t.Fatalf("reload consume record: %v", err)
	}
	if got.Status != ConsumeStatusSucceeded {
		t.Fatalf("unexpected succeeded record: %+v", got)
	}
}
