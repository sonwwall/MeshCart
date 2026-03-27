package service

import (
	"encoding/json"
	"strconv"

	logx "meshcart/app/log"
	mqx "meshcart/app/mq"
	dalmodel "meshcart/services/payment-service/dal/model"

	"go.uber.org/zap"
)

const (
	paymentSucceededEventName = "payment.succeeded"
	paymentServiceProducer    = "payment-service"
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

func (s *PaymentService) buildPaymentSucceededOutbox(payment *dalmodel.Payment, topic string) (*dalmodel.PaymentOutbox, error) {
	payload, err := json.Marshal(paymentSucceededPayload{
		PaymentID:      payment.PaymentID,
		OrderID:        payment.OrderID,
		UserID:         payment.UserID,
		Amount:         payment.Amount,
		PaymentMethod:  payment.PaymentMethod,
		PaymentTradeNo: payment.PaymentTradeNo,
		SucceededAt:    payment.SucceededAt.Unix(),
	})
	if err != nil {
		return nil, err
	}

	envelopeBody, err := mqx.MarshalEnvelope(mqx.Envelope{
		ID:         strconv.FormatInt(s.node.Generate().Int64(), 10),
		EventName:  paymentSucceededEventName,
		Topic:      topic,
		Key:        strconv.FormatInt(payment.PaymentID, 10),
		Producer:   paymentServiceProducer,
		Version:    1,
		OccurredAt: *payment.SucceededAt,
		Payload:    payload,
	})
	if err != nil {
		return nil, err
	}

	headersJSON, err := json.Marshal(map[string]string{
		"event_name": paymentSucceededEventName,
		"producer":   paymentServiceProducer,
	})
	if err != nil {
		return nil, err
	}

	record := &dalmodel.PaymentOutbox{
		ID:          s.node.Generate().Int64(),
		Topic:       topic,
		EventName:   paymentSucceededEventName,
		EventKey:    strconv.FormatInt(payment.PaymentID, 10),
		Producer:    paymentServiceProducer,
		HeadersJSON: headersJSON,
		Body:        envelopeBody,
		Status:      mqx.OutboxStatusPending,
		MaxRetries:  16,
	}
	logx.L(nil).Info("built payment succeeded outbox record",
		zap.Int64("payment_id", payment.PaymentID),
		zap.Int64("order_id", payment.OrderID),
		zap.Int64("outbox_id", record.ID),
		zap.String("topic", topic),
	)
	return record, nil
}
