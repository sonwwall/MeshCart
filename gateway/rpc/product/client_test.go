package product

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/server"

	basepb "meshcart/kitex_gen/meshcart/base"
	productpb "meshcart/kitex_gen/meshcart/product"
	productservice "meshcart/kitex_gen/meshcart/product/productservice"
)

type slowProductService struct {
	sleep   time.Duration
	started chan struct{}
}

func (s *slowProductService) CreateProduct(ctx context.Context, request *productpb.CreateProductRequest) (*productpb.CreateProductResponse, error) {
	return &productpb.CreateProductResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *slowProductService) CreateProductSaga(ctx context.Context, request *productpb.CreateProductSagaRequest) (*productpb.CreateProductResponse, error) {
	return &productpb.CreateProductResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *slowProductService) CompensateCreateProductSaga(ctx context.Context, request *productpb.CompensateCreateProductSagaRequest) (*productpb.CompensateCreateProductSagaResponse, error) {
	return &productpb.CompensateCreateProductSagaResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *slowProductService) UpdateProduct(ctx context.Context, request *productpb.UpdateProductRequest) (*productpb.UpdateProductResponse, error) {
	return &productpb.UpdateProductResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *slowProductService) ChangeProductStatus(ctx context.Context, request *productpb.ChangeProductStatusRequest) (*productpb.ChangeProductStatusResponse, error) {
	return &productpb.ChangeProductStatusResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *slowProductService) GetProductDetail(ctx context.Context, request *productpb.GetProductDetailRequest) (*productpb.GetProductDetailResponse, error) {
	select {
	case s.started <- struct{}{}:
	default:
	}
	time.Sleep(s.sleep)
	return &productpb.GetProductDetailResponse{
		Product: &productpb.Product{
			Id:        request.GetProductId(),
			Title:     "slow product",
			Status:    2,
			CreatorId: 1,
		},
		Base: &basepb.BaseResponse{Code: 0, Message: "成功"},
	}, nil
}

func (s *slowProductService) ListProducts(ctx context.Context, request *productpb.ListProductsRequest) (*productpb.ListProductsResponse, error) {
	return &productpb.ListProductsResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func (s *slowProductService) BatchGetSku(ctx context.Context, request *productpb.BatchGetSkuRequest) (*productpb.BatchGetSkuResponse, error) {
	return &productpb.BatchGetSkuResponse{Base: &basepb.BaseResponse{Code: 0, Message: "成功"}}, nil
}

func TestClient_GetProductDetailTimeout(t *testing.T) {
	addr := acquireFreeAddr(t)
	svc := &slowProductService{
		sleep:   200 * time.Millisecond,
		started: make(chan struct{}, 1),
	}
	svr := productservice.NewServer(svc, server.WithServiceAddr(addr))

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- svr.Run()
	}()

	waitForServer(t, addr.String())
	t.Cleanup(func() {
		if err := svr.Stop(); err != nil {
			t.Fatalf("stop test server: %v", err)
		}
		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Fatalf("server exited with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for test server shutdown")
		}
	})

	client, err := NewClient("ProductService", addr.String(), "direct", "", 100*time.Millisecond, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	start := time.Now()
	_, err = client.GetProductDetail(context.Background(), &productpb.GetProductDetailRequest{ProductId: 1001})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected rpc timeout error")
	}
	t.Logf("rpc returned expected timeout error: %v", err)
	if elapsed >= svc.sleep {
		t.Fatalf("expected timeout before server completed, elapsed=%s sleep=%s", elapsed, svc.sleep)
	}
	if elapsed > 150*time.Millisecond {
		t.Fatalf("expected timeout close to configured rpc timeout, got elapsed=%s", elapsed)
	}
	select {
	case <-svc.started:
	default:
		t.Fatal("expected server handler to start")
	}
	if !isTimeoutError(err) {
		t.Fatalf("expected timeout-like error, got %v", err)
	}
}

func TestClient_GetProductDetailConnectTimeout(t *testing.T) {
	client, err := newClientWithOptions(
		"ProductService",
		"127.0.0.1:1",
		"direct",
		"",
		50*time.Millisecond,
		2*time.Second,
		client.WithDialer(remote.SynthesizedDialer{
			DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
				time.Sleep(timeout + 20*time.Millisecond)
				return nil, context.DeadlineExceeded
			},
		}),
		client.WithFailureRetry(noRetryPolicy()),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	start := time.Now()
	_, err = client.GetProductDetail(context.Background(), &productpb.GetProductDetailRequest{ProductId: 1001})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected connect timeout error")
	}
	t.Logf("rpc returned expected connect timeout error: %v", err)
	if elapsed >= 2*time.Second {
		t.Fatalf("expected connect timeout before rpc timeout budget was exhausted, got elapsed=%s", elapsed)
	}
	if !isTimeoutError(err) {
		t.Fatalf("expected timeout-like error, got %v", err)
	}
}

func acquireFreeAddr(t *testing.T) *net.TCPAddr {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free addr: %v", err)
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected addr type %T", ln.Addr())
	}
	return addr
}

func waitForServer(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %s", addr)
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

func noRetryPolicy() *retry.FailurePolicy {
	p := retry.NewFailurePolicy()
	p.WithMaxRetryTimes(0)
	return p
}
