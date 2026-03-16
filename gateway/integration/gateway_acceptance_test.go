package integration

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"meshcart/app/common"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/handler"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	productrpc "meshcart/gateway/rpc/product"
	userrpc "meshcart/gateway/rpc/user"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	productpb "meshcart/kitex_gen/meshcart/product"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func TestGatewayAcceptance_LoginFlow(t *testing.T) {
	svcCtx := newTestServiceContext(t, config.RateLimitConfig{
		Enabled: true,
		GlobalIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		LoginIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		RegisterIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		AdminWriteUser: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		AdminWriteRoute: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		EntryTTL:        time.Minute,
		CleanupInterval: time.Minute,
	})
	svcCtx.UserClient = &stubUserClient{
		loginFn: func(_ context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			if req.Username != "tester" || req.Password != "123456" {
				t.Fatalf("unexpected login request: %+v", req)
			}
			return &userrpc.LoginResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   101,
				Username: "tester",
				Role:     authz.RoleUser,
			}, nil
		},
	}

	addr := startGatewayTestServer(t, svcCtx)
	traceID := "trace-login-acceptance"
	resp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/login", strings.NewReader(`{"username":"tester","password":"123456"}`), map[string]string{
		"Content-Type": "application/json",
		"X-Trace-Id":   traceID,
	})

	if resp.Code != common.CodeOK {
		t.Fatalf("expected code 0, got %d message=%q", resp.Code, resp.Message)
	}
	if resp.TraceID != traceID {
		t.Fatalf("expected trace_id %q, got %q", traceID, resp.TraceID)
	}

	var data struct {
		UserID   int64  `json:"user_id"`
		Username string `json:"username"`
		Role     string `json:"role"`
		Token    string `json:"token"`
	}
	decodeData(t, resp.Data, &data)

	if data.UserID != 101 {
		t.Fatalf("expected user_id 101, got %d", data.UserID)
	}
	if data.Username != "tester" {
		t.Fatalf("expected username tester, got %q", data.Username)
	}
	if data.Role != authz.RoleUser {
		t.Fatalf("expected role %q, got %q", authz.RoleUser, data.Role)
	}
	if !strings.HasPrefix(data.Token, "Bearer ") {
		t.Fatalf("expected bearer token, got %q", data.Token)
	}
}

func TestGatewayAcceptance_ProductListFlow(t *testing.T) {
	svcCtx := newTestServiceContext(t, config.RateLimitConfig{
		Enabled: true,
		GlobalIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		LoginIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		RegisterIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		AdminWriteUser: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		AdminWriteRoute: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		EntryTTL:        time.Minute,
		CleanupInterval: time.Minute,
	})
	svcCtx.ProductClient = &stubProductClient{
		listProductsFn: func(_ context.Context, req *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
			if req.GetPage() != 1 || req.GetPageSize() != 10 {
				t.Fatalf("unexpected list request: %+v", req)
			}
			if req.Status == nil || req.GetStatus() != 2 {
				t.Fatalf("expected online status filter, got %+v", req.Status)
			}
			return &productrpc.ListProductsResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Products: []*productpb.ProductListItem{
					{
						Id:           2001,
						Title:        "MeshCart Tee",
						SubTitle:     "Basic",
						CategoryId:   12,
						Brand:        "MeshCart",
						Status:       2,
						MinSalePrice: 1999,
						CoverUrl:     "https://example.test/cover.png",
					},
				},
				Total: 1,
			}, nil
		},
	}

	addr := startGatewayTestServer(t, svcCtx)
	traceID := "trace-product-list-acceptance"
	resp := doRequest(t, http.MethodGet, "http://"+addr+"/api/v1/products?page=1&page_size=10", nil, map[string]string{
		"X-Trace-Id": traceID,
	})

	if resp.Code != common.CodeOK {
		t.Fatalf("expected code 0, got %d message=%q", resp.Code, resp.Message)
	}
	if resp.TraceID != traceID {
		t.Fatalf("expected trace_id %q, got %q", traceID, resp.TraceID)
	}

	var data struct {
		Products []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"products"`
		Total int64 `json:"total"`
	}
	decodeData(t, resp.Data, &data)

	if data.Total != 1 {
		t.Fatalf("expected total 1, got %d", data.Total)
	}
	if len(data.Products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(data.Products))
	}
	if data.Products[0].ID != 2001 || data.Products[0].Title != "MeshCart Tee" {
		t.Fatalf("unexpected product payload: %+v", data.Products[0])
	}
}

func TestGatewayAcceptance_GlobalRateLimitWiring(t *testing.T) {
	svcCtx := newTestServiceContext(t, config.RateLimitConfig{
		Enabled: true,
		GlobalIP: config.RateLimitRuleConfig{
			RatePerSecond: 1,
			Burst:         1,
		},
		LoginIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		RegisterIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		AdminWriteUser: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		AdminWriteRoute: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		EntryTTL:        time.Minute,
		CleanupInterval: time.Minute,
	})
	svcCtx.ProductClient = &stubProductClient{
		listProductsFn: func(_ context.Context, _ *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
			return &productrpc.ListProductsResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				Products: []*productpb.ProductListItem{},
				Total:    0,
			}, nil
		},
		getProductDetailFn: func(_ context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{
					Id:         req.GetProductId(),
					Title:      "MeshCart Tee",
					Status:     2,
					Skus:       []*productpb.ProductSku{},
					CategoryId: 12,
				},
			}, nil
		},
	}

	addr := startGatewayTestServer(t, svcCtx)
	first := doRequest(t, http.MethodGet, "http://"+addr+"/api/v1/products?page=1&page_size=10", nil, nil)
	if first.Code != common.CodeOK {
		t.Fatalf("expected first request success, got code=%d message=%q", first.Code, first.Message)
	}

	second := doRequest(t, http.MethodGet, "http://"+addr+"/api/v1/products/detail/2001", nil, nil)
	if second.Code != common.CodeTooManyReq {
		t.Fatalf("expected second request to hit global rate limit, got code=%d message=%q", second.Code, second.Message)
	}
	if second.Message != common.ErrTooManyRequests.Msg {
		t.Fatalf("expected too many requests message, got %q", second.Message)
	}
}

func TestGatewayAcceptance_LoginTimeoutResponse(t *testing.T) {
	svcCtx := newTestServiceContext(t, permissiveRateLimitConfig())
	svcCtx.UserClient = &stubUserClient{
		loginFn: func(ctx context.Context, _ *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	addr := startGatewayTestServerWithTimeout(t, svcCtx, 50*time.Millisecond)
	traceID := "trace-login-timeout"
	resp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/login", strings.NewReader(`{"username":"tester","password":"123456"}`), map[string]string{
		"Content-Type": "application/json",
		"X-Trace-Id":   traceID,
	})

	if resp.Code != common.CodeInternalError {
		t.Fatalf("expected internal error code, got %d", resp.Code)
	}
	if resp.Message != common.ErrServiceBusy.Msg {
		t.Fatalf("expected service busy message, got %q", resp.Message)
	}
	if resp.TraceID != traceID {
		t.Fatalf("expected trace_id %q, got %q", traceID, resp.TraceID)
	}
}

func TestGatewayAcceptance_LoginUnavailableResponse(t *testing.T) {
	svcCtx := newTestServiceContext(t, permissiveRateLimitConfig())
	svcCtx.UserClient = &stubUserClient{
		loginFn: func(_ context.Context, _ *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return nil, errors.New("dial tcp 127.0.0.1:8888: connect: connection refused")
		},
	}

	addr := startGatewayTestServer(t, svcCtx)
	resp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/login", strings.NewReader(`{"username":"tester","password":"123456"}`), map[string]string{
		"Content-Type": "application/json",
	})

	if resp.Code != common.CodeInternalError {
		t.Fatalf("expected internal error code, got %d", resp.Code)
	}
	if resp.Message != common.ErrServiceUnavailable.Msg {
		t.Fatalf("expected service unavailable message, got %q", resp.Message)
	}
}

type stubUserClient struct {
	loginFn      func(context.Context, *userrpc.LoginRequest) (*userrpc.LoginResponse, error)
	registerFn   func(context.Context, *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error)
	getUserFn    func(context.Context, *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error)
	updateRoleFn func(context.Context, *userrpc.UpdateUserRoleRequest) (*userrpc.UpdateUserRoleResponse, error)
}

func (s *stubUserClient) Login(ctx context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
	if s.loginFn == nil {
		return nil, nil
	}
	return s.loginFn(ctx, req)
}

func (s *stubUserClient) Register(ctx context.Context, req *userrpc.RegisterRequest) (*userrpc.RegisterResponse, error) {
	if s.registerFn == nil {
		return &userrpc.RegisterResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.registerFn(ctx, req)
}

func (s *stubUserClient) GetUser(ctx context.Context, req *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error) {
	if s.getUserFn == nil {
		return &userrpc.GetUserResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.getUserFn(ctx, req)
}

func (s *stubUserClient) UpdateUserRole(ctx context.Context, req *userrpc.UpdateUserRoleRequest) (*userrpc.UpdateUserRoleResponse, error) {
	if s.updateRoleFn == nil {
		return &userrpc.UpdateUserRoleResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.updateRoleFn(ctx, req)
}

type stubProductClient struct {
	createProductFn               func(context.Context, *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error)
	createProductSagaFn           func(context.Context, *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error)
	compensateCreateProductSagaFn func(context.Context, *productpb.CompensateCreateProductSagaRequest) (*productrpc.ChangeProductStatusResponse, error)
	updateProductFn               func(context.Context, *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error)
	changeStatusFn                func(context.Context, *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error)
	getProductDetailFn            func(context.Context, *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error)
	listProductsFn                func(context.Context, *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error)
	batchGetSkuFn                 func(context.Context, *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error)
}

func (s *stubProductClient) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (*productrpc.CreateProductResponse, error) {
	if s.createProductFn == nil {
		return &productrpc.CreateProductResponse{Code: common.CodeOK, Message: "成功", ProductID: 1}, nil
	}
	return s.createProductFn(ctx, req)
}
func (s *stubProductClient) CreateProductSaga(ctx context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error) {
	if s.createProductSagaFn == nil {
		return &productrpc.CreateProductResponse{Code: common.CodeOK, Message: "成功", ProductID: 1}, nil
	}
	return s.createProductSagaFn(ctx, req)
}
func (s *stubProductClient) CompensateCreateProductSaga(ctx context.Context, req *productpb.CompensateCreateProductSagaRequest) (*productrpc.ChangeProductStatusResponse, error) {
	if s.compensateCreateProductSagaFn == nil {
		return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.compensateCreateProductSagaFn(ctx, req)
}

func (s *stubProductClient) UpdateProduct(ctx context.Context, req *productpb.UpdateProductRequest) (*productrpc.UpdateProductResponse, error) {
	if s.updateProductFn == nil {
		return &productrpc.UpdateProductResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.updateProductFn(ctx, req)
}

func (s *stubProductClient) ChangeProductStatus(ctx context.Context, req *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error) {
	if s.changeStatusFn == nil {
		return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.changeStatusFn(ctx, req)
}

func (s *stubProductClient) GetProductDetail(ctx context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
	if s.getProductDetailFn == nil {
		return &productrpc.GetProductDetailResponse{
			Code:    common.CodeOK,
			Message: "成功",
			Product: &productpb.Product{Id: req.GetProductId(), Title: "product"},
		}, nil
	}
	return s.getProductDetailFn(ctx, req)
}

func (s *stubProductClient) ListProducts(ctx context.Context, req *productpb.ListProductsRequest) (*productrpc.ListProductsResponse, error) {
	if s.listProductsFn == nil {
		return &productrpc.ListProductsResponse{Code: common.CodeOK, Message: "成功", Products: []*productpb.ProductListItem{}, Total: 0}, nil
	}
	return s.listProductsFn(ctx, req)
}

func (s *stubProductClient) BatchGetSKU(ctx context.Context, req *productpb.BatchGetSkuRequest) (*productrpc.BatchGetSKUResponse, error) {
	if s.batchGetSkuFn == nil {
		return &productrpc.BatchGetSKUResponse{Code: common.CodeOK, Message: "成功", Skus: []*productpb.ProductSku{}}, nil
	}
	return s.batchGetSkuFn(ctx, req)
}

type stubInventoryClient struct {
	getSkuStockFn                 func(context.Context, *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error)
	batchGetSkuStockFn            func(context.Context, *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error)
	checkSaleableStockFn          func(context.Context, *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error)
	initSkuStocksFn               func(context.Context, *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	initSkuStocksSagaFn           func(context.Context, *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error)
	compensateInitSkuStocksSagaFn func(context.Context, *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventoryrpc.CompensateInitSkuStocksResponse, error)
	freezeSkuStocksFn             func(context.Context, *inventorypb.FreezeSkuStocksRequest) (*inventoryrpc.FreezeSkuStocksResponse, error)
	adjustStockFn                 func(context.Context, *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error)
}

func (s *stubInventoryClient) GetSkuStock(ctx context.Context, req *inventorypb.GetSkuStockRequest) (*inventoryrpc.GetSkuStockResponse, error) {
	if s.getSkuStockFn == nil {
		return &inventoryrpc.GetSkuStockResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.getSkuStockFn(ctx, req)
}

func (s *stubInventoryClient) BatchGetSkuStock(ctx context.Context, req *inventorypb.BatchGetSkuStockRequest) (*inventoryrpc.BatchGetSkuStockResponse, error) {
	if s.batchGetSkuStockFn == nil {
		return &inventoryrpc.BatchGetSkuStockResponse{Code: common.CodeOK, Message: "成功", Stocks: []*inventorypb.SkuStock{}}, nil
	}
	return s.batchGetSkuStockFn(ctx, req)
}

func (s *stubInventoryClient) CheckSaleableStock(ctx context.Context, req *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
	if s.checkSaleableStockFn == nil {
		return &inventoryrpc.CheckSaleableStockResponse{Code: common.CodeOK, Message: "成功", Saleable: true, AvailableStock: 100}, nil
	}
	return s.checkSaleableStockFn(ctx, req)
}

func (s *stubInventoryClient) InitSkuStocks(ctx context.Context, req *inventorypb.InitSkuStocksRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
	if s.initSkuStocksFn == nil {
		return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功", Stocks: []*inventorypb.SkuStock{}}, nil
	}
	return s.initSkuStocksFn(ctx, req)
}
func (s *stubInventoryClient) InitSkuStocksSaga(ctx context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
	if s.initSkuStocksSagaFn == nil {
		return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功", Stocks: []*inventorypb.SkuStock{}}, nil
	}
	return s.initSkuStocksSagaFn(ctx, req)
}
func (s *stubInventoryClient) CompensateInitSkuStocksSaga(ctx context.Context, req *inventorypb.CompensateInitSkuStocksSagaRequest) (*inventoryrpc.CompensateInitSkuStocksResponse, error) {
	if s.compensateInitSkuStocksSagaFn == nil {
		return &inventoryrpc.CompensateInitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.compensateInitSkuStocksSagaFn(ctx, req)
}

func (s *stubInventoryClient) FreezeSkuStocks(ctx context.Context, req *inventorypb.FreezeSkuStocksRequest) (*inventoryrpc.FreezeSkuStocksResponse, error) {
	if s.freezeSkuStocksFn == nil {
		return &inventoryrpc.FreezeSkuStocksResponse{Code: common.CodeOK, Message: "成功", Stocks: []*inventorypb.SkuStock{}}, nil
	}
	return s.freezeSkuStocksFn(ctx, req)
}

func (s *stubInventoryClient) AdjustStock(ctx context.Context, req *inventorypb.AdjustStockRequest) (*inventoryrpc.AdjustStockResponse, error) {
	if s.adjustStockFn == nil {
		return &inventoryrpc.AdjustStockResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.adjustStockFn(ctx, req)
}

func newTestServiceContext(t *testing.T, rl config.RateLimitConfig) *svc.ServiceContext {
	t.Helper()

	jwtMiddleware, err := middleware.NewJWT(config.JWTConfig{
		Secret:            "integration-secret",
		Issuer:            "meshcart.gateway",
		TimeoutMinutes:    60,
		MaxRefreshMinutes: 120,
	})
	if err != nil {
		t.Fatalf("init jwt middleware: %v", err)
	}

	ac, err := authz.NewAccessController()
	if err != nil {
		t.Fatalf("init access controller: %v", err)
	}

	return &svc.ServiceContext{
		Config: config.Config{
			App: config.AppConfig{Name: "gateway", Env: "test"},
			JWT: config.JWTConfig{
				Secret:            "integration-secret",
				Issuer:            "meshcart.gateway",
				TimeoutMinutes:    60,
				MaxRefreshMinutes: 120,
			},
			RateLimit: rl,
		},
		UserClient:      &stubUserClient{},
		ProductClient:   &stubProductClient{},
		InventoryClient: &stubInventoryClient{},
		AccessControl:   ac,
		JWT:             jwtMiddleware,
		RateLimiter:     middleware.NewRateLimitStore(rl),
	}
}

func startGatewayTestServer(t *testing.T, svcCtx *svc.ServiceContext) string {
	t.Helper()
	return startGatewayTestServerWithTimeout(t, svcCtx, 0)
}

func startGatewayTestServerWithTimeout(t *testing.T, svcCtx *svc.ServiceContext, requestTimeout time.Duration) string {
	t.Helper()

	addr := reserveTCPAddr(t)
	h := server.New(server.WithHostPorts(addr))
	if requestTimeout > 0 {
		h.Use(middleware.RequestTimeout(requestTimeout))
	}
	handler.Register(h, svcCtx)

	go func() {
		_ = h.Run()
	}()
	waitForServer(t, addr)

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = h.Shutdown(shutdownCtx)
	})
	return addr
}

func permissiveRateLimitConfig() config.RateLimitConfig {
	return config.RateLimitConfig{
		Enabled: true,
		GlobalIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		LoginIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		RegisterIP: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		AdminWriteUser: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		AdminWriteRoute: config.RateLimitRuleConfig{
			RatePerSecond: 100,
			Burst:         100,
		},
		EntryTTL:        time.Minute,
		CleanupInterval: time.Minute,
	}
}

func doRequest(t *testing.T, method, url string, body io.Reader, headers map[string]string) common.HTTPResponse {
	t.Helper()

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	var httpResp common.HTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&httpResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return httpResp
}

func decodeData(t *testing.T, src interface{}, dst interface{}) {
	t.Helper()

	payload, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal response data: %v", err)
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		t.Fatalf("unmarshal response data: %v", err)
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	defer ln.Close()

	return ln.Addr().String()
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

	t.Fatalf("server did not start listening at %s", addr)
}
