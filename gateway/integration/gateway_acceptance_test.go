package integration

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"meshcart/app/common"
	"meshcart/gateway/config"
	"meshcart/gateway/internal/auth"
	"meshcart/gateway/internal/authz"
	"meshcart/gateway/internal/handler"
	"meshcart/gateway/internal/middleware"
	"meshcart/gateway/internal/svc"
	cartrpc "meshcart/gateway/rpc/cart"
	inventoryrpc "meshcart/gateway/rpc/inventory"
	orderrpc "meshcart/gateway/rpc/order"
	paymentrpc "meshcart/gateway/rpc/payment"
	productrpc "meshcart/gateway/rpc/product"
	userrpc "meshcart/gateway/rpc/user"
	cartpb "meshcart/kitex_gen/meshcart/cart"
	inventorypb "meshcart/kitex_gen/meshcart/inventory"
	orderpb "meshcart/kitex_gen/meshcart/order"
	paymentpb "meshcart/kitex_gen/meshcart/payment"
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
		UserID       int64  `json:"user_id"`
		Username     string `json:"username"`
		Role         string `json:"role"`
		SessionID    string `json:"session_id"`
		TokenType    string `json:"token_type"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
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
	if data.SessionID == "" {
		t.Fatal("expected session_id")
	}
	if data.TokenType != "Bearer" {
		t.Fatalf("expected token type Bearer, got %q", data.TokenType)
	}
	if !strings.HasPrefix(data.AccessToken, "Bearer ") {
		t.Fatalf("expected bearer access token, got %q", data.AccessToken)
	}
	if data.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}
}

func TestGatewayAcceptance_RefreshTokenFlow(t *testing.T) {
	svcCtx := newTestServiceContext(t, permissiveRateLimitConfig())
	var currentRole atomic.Value
	currentRole.Store(authz.RoleUser)

	svcCtx.UserClient = &stubUserClient{
		loginFn: func(_ context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return &userrpc.LoginResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   201,
				Username: req.Username,
				Role:     authz.RoleUser,
			}, nil
		},
		getUserFn: func(_ context.Context, req *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error) {
			return &userrpc.GetUserResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   req.UserID,
				Username: "tester",
				Role:     currentRole.Load().(string),
			}, nil
		},
	}

	addr := startGatewayTestServer(t, svcCtx)
	authData := loginAndGetAuthData(t, addr, "tester", "123456")
	currentRole.Store(authz.RoleAdmin)

	refreshResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/refresh_token", strings.NewReader(`{"refresh_token":"`+authData.RefreshToken+`"}`), map[string]string{
		"Content-Type": "application/json",
	})
	if refreshResp.Code != common.CodeOK {
		t.Fatalf("expected refresh success, got code=%d message=%q", refreshResp.Code, refreshResp.Message)
	}

	var refreshed struct {
		SessionID    string `json:"session_id"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	decodeData(t, refreshResp.Data, &refreshed)
	if refreshed.SessionID != authData.SessionID {
		t.Fatalf("expected same session id, got %q", refreshed.SessionID)
	}
	if refreshed.RefreshToken == authData.RefreshToken {
		t.Fatal("expected refresh token rotated")
	}

	meResp := doRequest(t, http.MethodGet, "http://"+addr+"/api/v1/user/me", nil, map[string]string{
		"Authorization": refreshed.AccessToken,
	})
	if meResp.Code != common.CodeOK {
		t.Fatalf("expected me success after refresh, got code=%d message=%q", meResp.Code, meResp.Message)
	}
	var me struct {
		Role string `json:"role"`
	}
	decodeData(t, meResp.Data, &me)
	if me.Role != authz.RoleAdmin {
		t.Fatalf("expected refreshed role %q, got %q", authz.RoleAdmin, me.Role)
	}

	oldRefreshResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/refresh_token", strings.NewReader(`{"refresh_token":"`+authData.RefreshToken+`"}`), map[string]string{
		"Content-Type": "application/json",
	})
	if oldRefreshResp.Code != common.CodeUnauthorized {
		t.Fatalf("expected old refresh token unauthorized, got code=%d message=%q", oldRefreshResp.Code, oldRefreshResp.Message)
	}
}

func TestGatewayAcceptance_LogoutInvalidatesRefreshToken(t *testing.T) {
	svcCtx := newTestServiceContext(t, permissiveRateLimitConfig())
	svcCtx.UserClient = &stubUserClient{
		loginFn: func(_ context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return &userrpc.LoginResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   301,
				Username: req.Username,
				Role:     authz.RoleUser,
			}, nil
		},
		getUserFn: func(_ context.Context, req *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error) {
			return &userrpc.GetUserResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   req.UserID,
				Username: "tester",
				Role:     authz.RoleUser,
			}, nil
		},
	}

	addr := startGatewayTestServer(t, svcCtx)
	authData := loginAndGetAuthData(t, addr, "tester", "123456")

	logoutResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/logout", strings.NewReader(`{}`), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": authData.AccessToken,
	})
	if logoutResp.Code != common.CodeOK {
		t.Fatalf("expected logout success, got code=%d message=%q", logoutResp.Code, logoutResp.Message)
	}

	refreshResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/refresh_token", strings.NewReader(`{"refresh_token":"`+authData.RefreshToken+`"}`), map[string]string{
		"Content-Type": "application/json",
	})
	if refreshResp.Code != common.CodeUnauthorized {
		t.Fatalf("expected refresh unauthorized after logout, got code=%d message=%q", refreshResp.Code, refreshResp.Message)
	}
}

func TestGatewayAcceptance_RefreshTokenConcurrentOnlyOneSucceeds(t *testing.T) {
	svcCtx := newTestServiceContext(t, permissiveRateLimitConfig())
	svcCtx.UserClient = &stubUserClient{
		loginFn: func(_ context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			return &userrpc.LoginResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   401,
				Username: req.Username,
				Role:     authz.RoleUser,
			}, nil
		},
		getUserFn: func(_ context.Context, req *userrpc.GetUserRequest) (*userrpc.GetUserResponse, error) {
			return &userrpc.GetUserResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   req.UserID,
				Username: "tester",
				Role:     authz.RoleUser,
			}, nil
		},
	}

	addr := startGatewayTestServer(t, svcCtx)
	authData := loginAndGetAuthData(t, addr, "tester", "123456")

	type result struct {
		code int32
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			resp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/refresh_token", strings.NewReader(`{"refresh_token":"`+authData.RefreshToken+`"}`), map[string]string{
				"Content-Type": "application/json",
			})
			results <- result{code: resp.Code}
		}()
	}

	successCount := 0
	unauthorizedCount := 0
	for i := 0; i < 2; i++ {
		resp := <-results
		switch resp.code {
		case common.CodeOK:
			successCount++
		case common.CodeUnauthorized:
			unauthorizedCount++
		default:
			t.Fatalf("unexpected refresh code %d", resp.code)
		}
	}
	if successCount != 1 || unauthorizedCount != 1 {
		t.Fatalf("expected 1 success and 1 unauthorized, got success=%d unauthorized=%d", successCount, unauthorizedCount)
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

func TestGatewayAcceptance_TradeFlow(t *testing.T) {
	svcCtx := newTestServiceContext(t, permissiveRateLimitConfig())

	var (
		productCreated       bool
		productOnline        bool
		inventoryInitialized bool
		cartAdded            bool
		orderCreated         bool
		paymentCreated       bool
		orderPaid            bool
		inventoryDeducted    bool
	)

	const (
		adminUserID = int64(9001)
		buyerUserID = int64(101)
		productID   = int64(2001)
		skuID       = int64(3001)
		orderID     = int64(4001)
		paymentID   = int64(5001)
	)

	svcCtx.UserClient = &stubUserClient{
		loginFn: func(_ context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			switch req.Username {
			case "admin":
				return &userrpc.LoginResponse{
					Code:     common.CodeOK,
					Message:  "成功",
					UserID:   adminUserID,
					Username: "admin",
					Role:     authz.RoleAdmin,
				}, nil
			case "buyer":
				return &userrpc.LoginResponse{
					Code:     common.CodeOK,
					Message:  "成功",
					UserID:   buyerUserID,
					Username: "buyer",
					Role:     authz.RoleUser,
				}, nil
			default:
				return &userrpc.LoginResponse{Code: 2010002, Message: "用户名或密码错误"}, nil
			}
		},
	}
	svcCtx.ProductClient = &stubProductClient{
		createProductSagaFn: func(_ context.Context, req *productpb.CreateProductSagaRequest) (*productrpc.CreateProductResponse, error) {
			if req.GetCreatorId() != adminUserID || req.GetTargetStatus() != 2 || len(req.GetSkus()) != 1 {
				t.Fatalf("unexpected create product saga req: %+v", req)
			}
			productCreated = true
			return &productrpc.CreateProductResponse{
				Code:      common.CodeOK,
				Message:   "成功",
				ProductID: productID,
				Skus: []*productpb.ProductSku{
					{Id: skuID, SpuId: productID, SkuCode: "meshcart-tee-blue-m", Title: "Blue M", SalePrice: 1999, Status: 1, CoverUrl: "https://example.test/tee-blue-m.png"},
				},
			}, nil
		},
		changeStatusFn: func(_ context.Context, req *productpb.ChangeProductStatusRequest) (*productrpc.ChangeProductStatusResponse, error) {
			if req.GetProductId() != productID || req.GetStatus() != 2 || !productCreated || !inventoryInitialized {
				t.Fatalf("unexpected change status req: %+v created=%v inventoryInitialized=%v", req, productCreated, inventoryInitialized)
			}
			productOnline = true
			return &productrpc.ChangeProductStatusResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
		getProductDetailFn: func(_ context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			if req.GetProductId() != productID {
				t.Fatalf("unexpected product detail req: %+v", req)
			}
			status := int32(1)
			if productOnline {
				status = 2
			}
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{
					Id:         productID,
					Title:      "MeshCart Tee",
					Status:     status,
					CategoryId: 12,
					Skus: []*productpb.ProductSku{
						{Id: skuID, SpuId: productID, SkuCode: "meshcart-tee-blue-m", Title: "Blue M", SalePrice: 1999, Status: 1, CoverUrl: "https://example.test/tee-blue-m.png"},
					},
				},
			}, nil
		},
	}
	svcCtx.InventoryClient = &stubInventoryClient{
		initSkuStocksSagaFn: func(_ context.Context, req *inventorypb.InitSkuStocksSagaRequest) (*inventoryrpc.InitSkuStocksResponse, error) {
			if !productCreated || len(req.GetStocks()) != 1 || req.GetStocks()[0].GetSkuId() != skuID || req.GetStocks()[0].GetTotalStock() != 10 {
				t.Fatalf("unexpected init sku stocks saga req: %+v created=%v", req, productCreated)
			}
			inventoryInitialized = true
			return &inventoryrpc.InitSkuStocksResponse{Code: common.CodeOK, Message: "成功"}, nil
		},
		checkSaleableStockFn: func(_ context.Context, req *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
			if !inventoryInitialized || !productOnline || req.GetSkuId() != skuID || req.GetQuantity() != 2 {
				t.Fatalf("unexpected check stock req: %+v inventoryInitialized=%v productOnline=%v", req, inventoryInitialized, productOnline)
			}
			return &inventoryrpc.CheckSaleableStockResponse{Code: common.CodeOK, Message: "成功", Saleable: true, AvailableStock: 10}, nil
		},
	}
	svcCtx.CartClient = &stubCartClient{
		addCartItemFn: func(_ context.Context, req *cartpb.AddCartItemRequest) (*cartrpc.AddCartItemResponse, error) {
			if !productOnline || req.GetUserId() != buyerUserID || req.GetProductId() != productID || req.GetSkuId() != skuID || req.GetQuantity() != 2 {
				t.Fatalf("unexpected add cart req: %+v productOnline=%v", req, productOnline)
			}
			cartAdded = true
			return &cartrpc.AddCartItemResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Item: &cartpb.CartItem{
					Id:                6001,
					UserId:            buyerUserID,
					ProductId:         productID,
					SkuId:             skuID,
					Quantity:          2,
					Checked:           true,
					TitleSnapshot:     "MeshCart Tee",
					SkuTitleSnapshot:  "Blue M",
					SalePriceSnapshot: 1999,
					CoverUrlSnapshot:  "https://example.test/tee-blue-m.png",
				},
			}, nil
		},
	}
	svcCtx.OrderClient = &stubOrderClient{
		createOrderFn: func(_ context.Context, req *orderpb.CreateOrderRequest) (*orderrpc.CreateOrderResponse, error) {
			if !cartAdded || req.GetUserId() != buyerUserID || req.GetRequestId() != "order-req-1" || len(req.GetItems()) != 1 {
				t.Fatalf("unexpected create order req: %+v cartAdded=%v", req, cartAdded)
			}
			orderCreated = true
			return &orderrpc.CreateOrderResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Order: &orderpb.Order{
					OrderId:     orderID,
					UserId:      buyerUserID,
					Status:      2,
					TotalAmount: 3998,
					PayAmount:   3998,
					ExpireAt:    time.Now().Add(30 * time.Minute).Unix(),
					Items: []*orderpb.OrderItem{
						{
							ItemId:               7001,
							OrderId:              orderID,
							ProductId:            productID,
							SkuId:                skuID,
							ProductTitleSnapshot: "MeshCart Tee",
							SkuTitleSnapshot:     "Blue M",
							SalePriceSnapshot:    1999,
							Quantity:             2,
							SubtotalAmount:       3998,
						},
					},
				},
			}, nil
		},
		getOrderFn: func(_ context.Context, req *orderpb.GetOrderRequest) (*orderrpc.GetOrderResponse, error) {
			if req.GetUserId() != buyerUserID || req.GetOrderId() != orderID {
				t.Fatalf("unexpected get order req: %+v", req)
			}
			status := int32(2)
			paymentIDText := ""
			paymentMethod := ""
			paymentTradeNo := ""
			paidAt := int64(0)
			if orderPaid {
				status = 3
				paymentIDText = "5001"
				paymentMethod = "mock"
				paymentTradeNo = "trade-5001"
				paidAt = time.Now().Unix()
			}
			return &orderrpc.GetOrderResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Order: &orderpb.Order{
					OrderId:        orderID,
					UserId:         buyerUserID,
					Status:         status,
					TotalAmount:    3998,
					PayAmount:      3998,
					ExpireAt:       time.Now().Add(30 * time.Minute).Unix(),
					PaymentId:      paymentIDText,
					PaymentMethod:  paymentMethod,
					PaymentTradeNo: paymentTradeNo,
					PaidAt:         paidAt,
					Items: []*orderpb.OrderItem{
						{
							ItemId:               7001,
							OrderId:              orderID,
							ProductId:            productID,
							SkuId:                skuID,
							ProductTitleSnapshot: "MeshCart Tee",
							SkuTitleSnapshot:     "Blue M",
							SalePriceSnapshot:    1999,
							Quantity:             2,
							SubtotalAmount:       3998,
						},
					},
				},
			}, nil
		},
	}
	svcCtx.PaymentClient = &stubPaymentClient{
		createPaymentFn: func(_ context.Context, req *paymentpb.CreatePaymentRequest) (*paymentrpc.CreatePaymentResponse, error) {
			if !orderCreated || req.GetUserId() != buyerUserID || req.GetOrderId() != orderID || req.GetPaymentMethod() != "mock" || req.GetRequestId() != "pay-req-1" {
				t.Fatalf("unexpected create payment req: %+v orderCreated=%v", req, orderCreated)
			}
			paymentCreated = true
			return &paymentrpc.CreatePaymentResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Payment: &paymentpb.Payment{
					PaymentId:     paymentID,
					OrderId:       orderID,
					UserId:        buyerUserID,
					Status:        1,
					PaymentMethod: "mock",
					Amount:        3998,
					Currency:      "CNY",
					ExpireAt:      time.Now().Add(15 * time.Minute).Unix(),
				},
			}, nil
		},
		confirmPaymentSuccessFn: func(_ context.Context, req *paymentpb.ConfirmPaymentSuccessRequest) (*paymentrpc.ConfirmPaymentSuccessResponse, error) {
			if !paymentCreated || req.GetPaymentId() != paymentID || req.GetPaymentMethod() != "mock" || req.GetRequestId() != "pay-confirm-1" || req.GetPaymentTradeNo() != "trade-5001" {
				t.Fatalf("unexpected confirm payment success req: %+v paymentCreated=%v", req, paymentCreated)
			}
			orderPaid = true
			inventoryDeducted = true
			return &paymentrpc.ConfirmPaymentSuccessResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Payment: &paymentpb.Payment{
					PaymentId:      paymentID,
					OrderId:        orderID,
					UserId:         buyerUserID,
					Status:         2,
					PaymentMethod:  "mock",
					Amount:         3998,
					Currency:       "CNY",
					PaymentTradeNo: "trade-5001",
					ExpireAt:       time.Now().Add(15 * time.Minute).Unix(),
					SucceededAt:    time.Now().Unix(),
				},
			}, nil
		},
		getPaymentFn: func(_ context.Context, req *paymentpb.GetPaymentRequest) (*paymentrpc.GetPaymentResponse, error) {
			if req.GetUserId() != buyerUserID || req.GetPaymentId() != paymentID {
				t.Fatalf("unexpected get payment req: %+v", req)
			}
			status := int32(1)
			tradeNo := ""
			succeededAt := int64(0)
			if orderPaid {
				status = 2
				tradeNo = "trade-5001"
				succeededAt = time.Now().Unix()
			}
			return &paymentrpc.GetPaymentResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Payment: &paymentpb.Payment{
					PaymentId:      paymentID,
					OrderId:        orderID,
					UserId:         buyerUserID,
					Status:         status,
					PaymentMethod:  "mock",
					Amount:         3998,
					Currency:       "CNY",
					PaymentTradeNo: tradeNo,
					ExpireAt:       time.Now().Add(15 * time.Minute).Unix(),
					SucceededAt:    succeededAt,
				},
			}, nil
		},
	}

	addr := startGatewayTestServer(t, svcCtx)

	adminToken := loginAndGetToken(t, addr, "admin", "123456")
	userToken := loginAndGetToken(t, addr, "buyer", "123456")

	createProductResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/admin/products", strings.NewReader(`{"title":"MeshCart Tee","category_id":12,"brand":"MeshCart","description":"Basic tee","status":2,"skus":[{"sku_code":"meshcart-tee-blue-m","title":"Blue M","sale_price":1999,"market_price":2599,"status":1,"cover_url":"https://example.test/tee-blue-m.png","initial_stock":10}]}`), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": adminToken,
	})
	if createProductResp.Code != common.CodeOK {
		t.Fatalf("expected create product success, got code=%d message=%q", createProductResp.Code, createProductResp.Message)
	}

	addCartResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/cart/items", strings.NewReader(`{"product_id":2001,"sku_id":3001,"quantity":2,"checked":true}`), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": userToken,
	})
	if addCartResp.Code != common.CodeOK {
		t.Fatalf("expected add cart success, got code=%d message=%q", addCartResp.Code, addCartResp.Message)
	}

	createOrderResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/orders", strings.NewReader(`{"request_id":"order-req-1","items":[{"product_id":2001,"sku_id":3001,"quantity":2}]}`), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": userToken,
	})
	if createOrderResp.Code != common.CodeOK {
		t.Fatalf("expected create order success, got code=%d message=%q", createOrderResp.Code, createOrderResp.Message)
	}

	createPaymentResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/payments", strings.NewReader(`{"order_id":4001,"payment_method":"mock","request_id":"pay-req-1"}`), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": userToken,
	})
	if createPaymentResp.Code != common.CodeOK {
		t.Fatalf("expected create payment success, got code=%d message=%q", createPaymentResp.Code, createPaymentResp.Message)
	}

	mockPayResp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/payments/5001/mock_success", strings.NewReader(`{"request_id":"pay-confirm-1","payment_trade_no":"trade-5001"}`), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": userToken,
	})
	if mockPayResp.Code != common.CodeOK {
		t.Fatalf("expected mock pay success, got code=%d message=%q", mockPayResp.Code, mockPayResp.Message)
	}

	getOrderResp := doRequest(t, http.MethodGet, "http://"+addr+"/api/v1/orders/4001", nil, map[string]string{
		"Authorization": userToken,
	})
	if getOrderResp.Code != common.CodeOK {
		t.Fatalf("expected get order success, got code=%d message=%q", getOrderResp.Code, getOrderResp.Message)
	}
	var orderData struct {
		OrderID        int64  `json:"order_id"`
		Status         int32  `json:"status"`
		PaymentID      string `json:"payment_id"`
		PaymentMethod  string `json:"payment_method"`
		PaymentTradeNo string `json:"payment_trade_no"`
	}
	decodeData(t, getOrderResp.Data, &orderData)
	if orderData.OrderID != orderID || orderData.Status != 3 || orderData.PaymentID != "5001" || orderData.PaymentMethod != "mock" || orderData.PaymentTradeNo != "trade-5001" {
		t.Fatalf("unexpected paid order payload: %+v", orderData)
	}

	getPaymentResp := doRequest(t, http.MethodGet, "http://"+addr+"/api/v1/payments/5001", nil, map[string]string{
		"Authorization": userToken,
	})
	if getPaymentResp.Code != common.CodeOK {
		t.Fatalf("expected get payment success, got code=%d message=%q", getPaymentResp.Code, getPaymentResp.Message)
	}
	var paymentData struct {
		PaymentID      int64  `json:"payment_id"`
		Status         int32  `json:"status"`
		PaymentMethod  string `json:"payment_method"`
		PaymentTradeNo string `json:"payment_trade_no"`
	}
	decodeData(t, getPaymentResp.Data, &paymentData)
	if paymentData.PaymentID != paymentID || paymentData.Status != 2 || paymentData.PaymentMethod != "mock" || paymentData.PaymentTradeNo != "trade-5001" {
		t.Fatalf("unexpected succeeded payment payload: %+v", paymentData)
	}

	if !productCreated || !productOnline || !inventoryInitialized || !cartAdded || !orderCreated || !paymentCreated || !orderPaid || !inventoryDeducted {
		t.Fatalf("unexpected flow flags productCreated=%v productOnline=%v inventoryInitialized=%v cartAdded=%v orderCreated=%v paymentCreated=%v orderPaid=%v inventoryDeducted=%v",
			productCreated, productOnline, inventoryInitialized, cartAdded, orderCreated, paymentCreated, orderPaid, inventoryDeducted)
	}
}

func TestGatewayAcceptance_CartUpdateBlockedByInsufficientStock(t *testing.T) {
	svcCtx := newTestServiceContext(t, permissiveRateLimitConfig())

	updateCalled := false
	svcCtx.UserClient = &stubUserClient{
		loginFn: func(_ context.Context, req *userrpc.LoginRequest) (*userrpc.LoginResponse, error) {
			if req.Username != "buyer" {
				t.Fatalf("unexpected login request: %+v", req)
			}
			return &userrpc.LoginResponse{
				Code:     common.CodeOK,
				Message:  "成功",
				UserID:   101,
				Username: "buyer",
				Role:     authz.RoleUser,
			}, nil
		},
	}
	svcCtx.CartClient = &stubCartClient{
		getCartFn: func(_ context.Context, req *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error) {
			if req.GetUserId() != 101 {
				t.Fatalf("unexpected get cart req: %+v", req)
			}
			return &cartrpc.GetCartResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Items: []*cartpb.CartItem{
					{Id: 11, UserId: 101, ProductId: 2001, SkuId: 3001, Quantity: 1, Checked: true},
				},
			}, nil
		},
		updateCartItemFn: func(_ context.Context, req *cartpb.UpdateCartItemRequest) (*cartrpc.UpdateCartItemResponse, error) {
			updateCalled = true
			return &cartrpc.UpdateCartItemResponse{Code: common.CodeOK, Message: "成功", Item: &cartpb.CartItem{Id: req.GetItemId(), UserId: req.GetUserId(), Quantity: req.GetQuantity()}}, nil
		},
	}
	svcCtx.ProductClient = &stubProductClient{
		getProductDetailFn: func(_ context.Context, req *productpb.GetProductDetailRequest) (*productrpc.GetProductDetailResponse, error) {
			if req.GetProductId() != 2001 {
				t.Fatalf("unexpected product detail req: %+v", req)
			}
			return &productrpc.GetProductDetailResponse{
				Code:    common.CodeOK,
				Message: "成功",
				Product: &productpb.Product{
					Id:     2001,
					Title:  "MeshCart Tee",
					Status: 2,
					Skus: []*productpb.ProductSku{
						{Id: 3001, Status: 1, Title: "Blue M"},
					},
				},
			}, nil
		},
	}
	svcCtx.InventoryClient = &stubInventoryClient{
		checkSaleableStockFn: func(_ context.Context, req *inventorypb.CheckSaleableStockRequest) (*inventoryrpc.CheckSaleableStockResponse, error) {
			if req.GetSkuId() != 3001 || req.GetQuantity() != 5 {
				t.Fatalf("unexpected stock check req: %+v", req)
			}
			return &inventoryrpc.CheckSaleableStockResponse{Code: 2050002, Message: "库存不足", Saleable: false, AvailableStock: 1}, nil
		},
	}

	addr := startGatewayTestServer(t, svcCtx)
	userToken := loginAndGetToken(t, addr, "buyer", "123456")

	resp := doRequest(t, http.MethodPut, "http://"+addr+"/api/v1/cart/items/11", strings.NewReader(`{"quantity":5}`), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": userToken,
	})
	if resp.Code != 2050002 {
		t.Fatalf("expected insufficient stock code, got code=%d message=%q", resp.Code, resp.Message)
	}
	if updateCalled {
		t.Fatal("expected cart update not to be called when stock check fails")
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
	reserveSkuStocksFn            func(context.Context, *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error)
	releaseReservedSkuStocksFn    func(context.Context, *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error)
	confirmDeductReservedFn       func(context.Context, *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error)
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

func (s *stubInventoryClient) ReserveSkuStocks(ctx context.Context, req *inventorypb.ReserveSkuStocksRequest) (*inventoryrpc.ReserveSkuStocksResponse, error) {
	if s.reserveSkuStocksFn == nil {
		return &inventoryrpc.ReserveSkuStocksResponse{Code: common.CodeOK, Message: "成功", Stocks: []*inventorypb.SkuStock{}}, nil
	}
	return s.reserveSkuStocksFn(ctx, req)
}

func (s *stubInventoryClient) ReleaseReservedSkuStocks(ctx context.Context, req *inventorypb.ReleaseReservedSkuStocksRequest) (*inventoryrpc.ReleaseReservedSkuStocksResponse, error) {
	if s.releaseReservedSkuStocksFn == nil {
		return &inventoryrpc.ReleaseReservedSkuStocksResponse{Code: common.CodeOK, Message: "成功", Stocks: []*inventorypb.SkuStock{}}, nil
	}
	return s.releaseReservedSkuStocksFn(ctx, req)
}

func (s *stubInventoryClient) ConfirmDeductReservedSkuStocks(ctx context.Context, req *inventorypb.ConfirmDeductReservedSkuStocksRequest) (*inventoryrpc.ConfirmDeductReservedSkuStocksResponse, error) {
	if s.confirmDeductReservedFn == nil {
		return &inventoryrpc.ConfirmDeductReservedSkuStocksResponse{Code: common.CodeOK, Message: "成功", Stocks: []*inventorypb.SkuStock{}}, nil
	}
	return s.confirmDeductReservedFn(ctx, req)
}

type stubCartClient struct {
	getCartFn        func(context.Context, *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error)
	addCartItemFn    func(context.Context, *cartpb.AddCartItemRequest) (*cartrpc.AddCartItemResponse, error)
	updateCartItemFn func(context.Context, *cartpb.UpdateCartItemRequest) (*cartrpc.UpdateCartItemResponse, error)
	removeCartItemFn func(context.Context, *cartpb.RemoveCartItemRequest) (*cartrpc.RemoveCartItemResponse, error)
	clearCartFn      func(context.Context, *cartpb.ClearCartRequest) (*cartrpc.ClearCartResponse, error)
}

func (s *stubCartClient) GetCart(ctx context.Context, req *cartpb.GetCartRequest) (*cartrpc.GetCartResponse, error) {
	if s.getCartFn == nil {
		return &cartrpc.GetCartResponse{Code: common.CodeOK, Message: "成功", Items: []*cartpb.CartItem{}}, nil
	}
	return s.getCartFn(ctx, req)
}

func (s *stubCartClient) AddCartItem(ctx context.Context, req *cartpb.AddCartItemRequest) (*cartrpc.AddCartItemResponse, error) {
	if s.addCartItemFn == nil {
		return &cartrpc.AddCartItemResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.addCartItemFn(ctx, req)
}

func (s *stubCartClient) UpdateCartItem(ctx context.Context, req *cartpb.UpdateCartItemRequest) (*cartrpc.UpdateCartItemResponse, error) {
	if s.updateCartItemFn == nil {
		return &cartrpc.UpdateCartItemResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.updateCartItemFn(ctx, req)
}

func (s *stubCartClient) RemoveCartItem(ctx context.Context, req *cartpb.RemoveCartItemRequest) (*cartrpc.RemoveCartItemResponse, error) {
	if s.removeCartItemFn == nil {
		return &cartrpc.RemoveCartItemResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.removeCartItemFn(ctx, req)
}

func (s *stubCartClient) ClearCart(ctx context.Context, req *cartpb.ClearCartRequest) (*cartrpc.ClearCartResponse, error) {
	if s.clearCartFn == nil {
		return &cartrpc.ClearCartResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.clearCartFn(ctx, req)
}

type stubOrderClient struct {
	createOrderFn func(context.Context, *orderpb.CreateOrderRequest) (*orderrpc.CreateOrderResponse, error)
	getOrderFn    func(context.Context, *orderpb.GetOrderRequest) (*orderrpc.GetOrderResponse, error)
	listOrdersFn  func(context.Context, *orderpb.ListOrdersRequest) (*orderrpc.ListOrdersResponse, error)
	cancelOrderFn func(context.Context, *orderpb.CancelOrderRequest) (*orderrpc.CancelOrderResponse, error)
}

func (s *stubOrderClient) CreateOrder(ctx context.Context, req *orderpb.CreateOrderRequest) (*orderrpc.CreateOrderResponse, error) {
	if s.createOrderFn == nil {
		return &orderrpc.CreateOrderResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.createOrderFn(ctx, req)
}

func (s *stubOrderClient) GetOrder(ctx context.Context, req *orderpb.GetOrderRequest) (*orderrpc.GetOrderResponse, error) {
	if s.getOrderFn == nil {
		return &orderrpc.GetOrderResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.getOrderFn(ctx, req)
}

func (s *stubOrderClient) ListOrders(ctx context.Context, req *orderpb.ListOrdersRequest) (*orderrpc.ListOrdersResponse, error) {
	if s.listOrdersFn == nil {
		return &orderrpc.ListOrdersResponse{Code: common.CodeOK, Message: "成功", Orders: []*orderpb.Order{}, Total: 0}, nil
	}
	return s.listOrdersFn(ctx, req)
}

func (s *stubOrderClient) CancelOrder(ctx context.Context, req *orderpb.CancelOrderRequest) (*orderrpc.CancelOrderResponse, error) {
	if s.cancelOrderFn == nil {
		return &orderrpc.CancelOrderResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.cancelOrderFn(ctx, req)
}

type stubPaymentClient struct {
	createPaymentFn         func(context.Context, *paymentpb.CreatePaymentRequest) (*paymentrpc.CreatePaymentResponse, error)
	getPaymentFn            func(context.Context, *paymentpb.GetPaymentRequest) (*paymentrpc.GetPaymentResponse, error)
	listPaymentsByOrderFn   func(context.Context, *paymentpb.ListPaymentsByOrderRequest) (*paymentrpc.ListPaymentsByOrderResponse, error)
	confirmPaymentSuccessFn func(context.Context, *paymentpb.ConfirmPaymentSuccessRequest) (*paymentrpc.ConfirmPaymentSuccessResponse, error)
	closePaymentFn          func(context.Context, *paymentpb.ClosePaymentRequest) (*paymentrpc.ClosePaymentResponse, error)
}

func (s *stubPaymentClient) CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (*paymentrpc.CreatePaymentResponse, error) {
	if s.createPaymentFn == nil {
		return &paymentrpc.CreatePaymentResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.createPaymentFn(ctx, req)
}

func (s *stubPaymentClient) GetPayment(ctx context.Context, req *paymentpb.GetPaymentRequest) (*paymentrpc.GetPaymentResponse, error) {
	if s.getPaymentFn == nil {
		return &paymentrpc.GetPaymentResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.getPaymentFn(ctx, req)
}

func (s *stubPaymentClient) ListPaymentsByOrder(ctx context.Context, req *paymentpb.ListPaymentsByOrderRequest) (*paymentrpc.ListPaymentsByOrderResponse, error) {
	if s.listPaymentsByOrderFn == nil {
		return &paymentrpc.ListPaymentsByOrderResponse{Code: common.CodeOK, Message: "成功", Payments: []*paymentpb.Payment{}}, nil
	}
	return s.listPaymentsByOrderFn(ctx, req)
}

func (s *stubPaymentClient) ConfirmPaymentSuccess(ctx context.Context, req *paymentpb.ConfirmPaymentSuccessRequest) (*paymentrpc.ConfirmPaymentSuccessResponse, error) {
	if s.confirmPaymentSuccessFn == nil {
		return &paymentrpc.ConfirmPaymentSuccessResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.confirmPaymentSuccessFn(ctx, req)
}

func (s *stubPaymentClient) ClosePayment(ctx context.Context, req *paymentpb.ClosePaymentRequest) (*paymentrpc.ClosePaymentResponse, error) {
	if s.closePaymentFn == nil {
		return &paymentrpc.ClosePaymentResponse{Code: common.CodeOK, Message: "成功"}, nil
	}
	return s.closePaymentFn(ctx, req)
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
			AuthSession: config.AuthSessionConfig{
				KeyPrefix:         "auth:session",
				RefreshTokenTTL:   24 * time.Hour,
				StoreTimeout:      time.Second,
				AccessTokenLeeway: 30 * time.Second,
			},
			JWT: config.JWTConfig{
				Secret:            "integration-secret",
				Issuer:            "meshcart.gateway",
				TimeoutMinutes:    60,
				MaxRefreshMinutes: 120,
			},
			RateLimit: rl,
		},
		UserClient:      &stubUserClient{},
		CartClient:      &stubCartClient{},
		OrderClient:     &stubOrderClient{},
		PaymentClient:   &stubPaymentClient{},
		ProductClient:   &stubProductClient{},
		InventoryClient: &stubInventoryClient{},
		AccessControl:   ac,
		JWT:             jwtMiddleware,
		SessionStore:    auth.NewMemorySessionStore(),
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

type loginAuthData struct {
	SessionID    string
	AccessToken  string
	RefreshToken string
}

func loginAndGetAuthData(t *testing.T, addr, username, password string) loginAuthData {
	t.Helper()

	resp := doRequest(t, http.MethodPost, "http://"+addr+"/api/v1/user/login", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`), map[string]string{
		"Content-Type": "application/json",
	})
	if resp.Code != common.CodeOK {
		t.Fatalf("expected login success for %s, got code=%d message=%q", username, resp.Code, resp.Message)
	}
	var data struct {
		SessionID    string `json:"session_id"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	decodeData(t, resp.Data, &data)
	if !strings.HasPrefix(data.AccessToken, "Bearer ") {
		t.Fatalf("expected bearer token for %s, got %q", username, data.AccessToken)
	}
	if data.RefreshToken == "" {
		t.Fatalf("expected refresh token for %s", username)
	}
	return loginAuthData{
		SessionID:    data.SessionID,
		AccessToken:  data.AccessToken,
		RefreshToken: data.RefreshToken,
	}
}

func loginAndGetToken(t *testing.T, addr, username, password string) string {
	t.Helper()
	return loginAndGetAuthData(t, addr, username, password).AccessToken
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
